/*
Copyright 2022 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kustomize

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	securefs "github.com/fluxcd/pkg/kustomize/filesys"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/hashicorp/go-multierror"
)

const (
	specField            = "spec"
	targetNSField        = "targetNamespace"
	patchesField         = "patches"
	componentsField      = "components"
	patchesSMField       = "patchesStrategicMerge"
	patchesJson6902Field = "patchesJson6902"
	imagesField          = "images"
)

// Action is the action that was taken on the kustomization file
type Action string

const (
	// CreatedAction is the action that was taken when creating the kustomization file
	CreatedAction Action = "created"
	// UnchangedAction is the action that was taken when the kustomization file was unchanged
	UnchangedAction Action = "unchanged"
)

// Generator is a kustomize generator
// It is responsible for generating a kustomization.yaml file from
// - a directory path and a kustomization object
type Generator struct {
	root          string
	kustomization unstructured.Unstructured
}

// SavingOptions is a function that can be used to apply saving options to a kustomization
type SavingOptions func(dirPath, file string, action Action) error

// NewGenerator creates a new kustomize generator
// It takes a root directory and a kustomization object
// If the root is empty, no enforcement of the root directory will be done when handling paths.
func NewGenerator(root string, kustomization unstructured.Unstructured) *Generator {
	return &Generator{
		root:          root,
		kustomization: kustomization,
	}
}

// WithSaveOriginalKustomization will save the original kustomization file
func WithSaveOriginalKustomization() SavingOptions {
	return func(dirPath, kfile string, action Action) error {
		// copy the original kustomization.yaml to the directory if we did not create it
		if action != CreatedAction {
			if err := copyFile(kfile, filepath.Join(dirPath, fmt.Sprint(path.Base(kfile), ".original"))); err != nil {
				errf := CleanDirectory(dirPath, action)
				return fmt.Errorf("%v %v", err, errf)
			}
		}
		return nil
	}
}

// WriteFile generates a kustomization.yaml in the given directory if it does not exist.
// It apply the flux kustomize resources to the kustomization.yaml and then write the
// updated kustomization.yaml to the directory.
// It returns an action that indicates if the kustomization.yaml was created or not.
// It is the caller's responsability to clean up the directory by using the provided function CleanDirectory.
// example:
// err := CleanDirectory(dirPath, action)
//
//	if err != nil {
//		log.Fatal(err)
//	}
func (g *Generator) WriteFile(dirPath string, opts ...SavingOptions) (Action, error) {
	action, kfile, err := g.generateKustomization(dirPath)
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("%v %v", err, errf)
	}

	data, err := os.ReadFile(kfile)
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("%w %s", err, errf)
	}

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	if err := yaml.Unmarshal(data, &kus); err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("%v %v", err, errf)
	}

	tg, ok, err := g.getNestedString(specField, targetNSField)
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("%v %v", err, errf)
	}
	if ok {
		kus.Namespace = tg
	}

	patches, err := g.getPatches()
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("unable to get patches: %w", fmt.Errorf("%v %v", err, errf))
	}

	for _, p := range patches {
		kus.Patches = append(kus.Patches, kustypes.Patch{
			Patch:  p.Patch,
			Target: adaptSelector(&p.Target),
		})
	}

	components, _, err := g.getNestedStringSlice(specField, componentsField)
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("unable to get components: %w", fmt.Errorf("%v %v", err, errf))
	}

	for _, component := range components {
		if !IsLocalRelativePath(component) {
			return "", fmt.Errorf("component path '%s' must be local and relative", component)
		}
		kus.Components = append(kus.Components, component)
	}

	patchesSM, err := g.getPatchesStrategicMerge()
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("unable to get patchesStrategicMerge: %w", fmt.Errorf("%v %v", err, errf))
	}

	for _, p := range patchesSM {
		kus.PatchesStrategicMerge = append(kus.PatchesStrategicMerge, kustypes.PatchStrategicMerge(p.Raw))
	}

	patchesJSON, err := g.getPatchesJson6902()
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("unable to get patchesJson6902: %w", fmt.Errorf("%v %v", err, errf))
	}

	for _, p := range patchesJSON {
		patch, err := json.Marshal(p.Patch)
		if err != nil {
			errf := CleanDirectory(dirPath, action)
			return action, fmt.Errorf("%v %v", err, errf)
		}
		kus.PatchesJson6902 = append(kus.PatchesJson6902, kustypes.Patch{
			Patch:  string(patch),
			Target: adaptSelector(&p.Target),
		})
	}

	images, err := g.getImages()
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("unable to get images: %w", fmt.Errorf("%v %v", err, errf))
	}

	for _, image := range images {
		newImage := kustypes.Image{
			Name:    image.Name,
			NewName: image.NewName,
			NewTag:  image.NewTag,
			Digest:  image.Digest,
		}
		if exists, index := checkKustomizeImageExists(kus.Images, image.Name); exists {
			kus.Images[index] = newImage
		} else {
			kus.Images = append(kus.Images, newImage)
		}
	}

	manifest, err := yaml.Marshal(kus)
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("%v %v", err, errf)
	}

	// copy the original kustomization.yaml to the directory if we did not create it
	for _, opt := range opts {
		if err := opt(dirPath, kfile, action); err != nil {
			return action, fmt.Errorf("failed to save original kustomization.yaml: %w", err)
		}
	}

	err = os.WriteFile(kfile, manifest, os.ModePerm)
	if err != nil {
		errf := CleanDirectory(dirPath, action)
		return action, fmt.Errorf("%v %v", err, errf)
	}

	return action, nil
}

func (g *Generator) getPatches() ([]kustomize.Patch, error) {
	patches, ok, err := g.getNestedSlice(specField, patchesField)
	if err != nil {
		return nil, err
	}

	var resultErr error
	if ok {
		res := make([]kustomize.Patch, 0, len(patches))
		for k, p := range patches {
			patch, ok := p.(map[string]interface{})
			if !ok {
				err := fmt.Errorf("unable to convert patch %d to map[string]interface{}", k)
				resultErr = multierror.Append(resultErr, err)
			}
			var kpatch kustomize.Patch
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(patch, &kpatch)
			if err != nil {
				resultErr = multierror.Append(resultErr, err)
			}
			res = append(res, kpatch)
		}
		return res, resultErr
	}

	return nil, resultErr

}

func (g *Generator) getPatchesStrategicMerge() ([]apiextensionsv1.JSON, error) {
	patches, ok, err := g.getNestedSlice(specField, patchesSMField)
	if err != nil {
		return nil, err
	}

	var resultErr error
	if ok {
		res := make([]apiextensionsv1.JSON, 0, len(patches))
		for k, p := range patches {
			patch, ok := p.(map[string]interface{})
			if !ok {
				err := fmt.Errorf("unable to convert patch %d to map[string]interface{}", k)
				resultErr = multierror.Append(resultErr, err)
			}
			var kpatch apiextensionsv1.JSON
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(patch, &kpatch)
			if err != nil {
				resultErr = multierror.Append(resultErr, err)
			}
			res = append(res, kpatch)
		}
		return res, resultErr
	}

	return nil, resultErr

}

func (g *Generator) getPatchesJson6902() ([]kustomize.JSON6902Patch, error) {
	patches, ok, err := g.getNestedSlice(specField, patchesJson6902Field)
	if err != nil {
		return nil, err
	}

	var resultErr error
	if ok {
		res := make([]kustomize.JSON6902Patch, 0, len(patches))
		for k, p := range patches {
			patch, ok := p.(map[string]interface{})
			if !ok {
				err := fmt.Errorf("unable to convert patch %d to map[string]interface{}", k)
				resultErr = multierror.Append(resultErr, err)
			}
			var kpatch kustomize.JSON6902Patch
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(patch, &kpatch)
			if err != nil {
				resultErr = multierror.Append(resultErr, err)
			}
			res = append(res, kpatch)
		}
		return res, resultErr
	}

	return nil, resultErr

}

func (g *Generator) getImages() ([]kustomize.Image, error) {
	img, ok, err := g.getNestedSlice(specField, imagesField)
	if err != nil {
		return nil, err
	}

	var resultErr error
	if ok {
		res := make([]kustomize.Image, 0, len(img))
		for k, i := range img {
			im, ok := i.(map[string]interface{})
			if !ok {
				err := fmt.Errorf("unable to convert patch %d to map[string]interface{}", k)
				resultErr = multierror.Append(resultErr, err)
			}
			var image kustomize.Image
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(im, &image)
			if err != nil {
				resultErr = multierror.Append(resultErr, err)
			}
			res = append(res, image)
		}
		return res, resultErr
	}

	return nil, resultErr

}

func checkKustomizeImageExists(images []kustypes.Image, imageName string) (bool, int) {
	for i, image := range images {
		if imageName == image.Name {
			return true, i
		}
	}

	return false, -1
}

func (g *Generator) getNestedString(fields ...string) (string, bool, error) {
	val, ok, err := unstructured.NestedString(g.kustomization.Object, fields...)
	if err != nil {
		return "", ok, err
	}

	return val, ok, nil
}

func (g *Generator) getNestedStringSlice(fields ...string) ([]string, bool, error) {
	val, ok, err := unstructured.NestedStringSlice(g.kustomization.Object, fields...)
	if err != nil {
		return []string{}, ok, err
	}

	return val, ok, nil
}

func (g *Generator) getNestedSlice(fields ...string) ([]interface{}, bool, error) {
	val, ok, err := unstructured.NestedSlice(g.kustomization.Object, fields...)
	if err != nil {
		return nil, ok, err
	}

	return val, ok, nil
}

func (g *Generator) generateKustomization(dirPath string) (Action, string, error) {
	var (
		err error
		fs  filesys.FileSystem
	)
	// use securefs only if the path is specified
	// otherwise, use the default filesystem.
	if g.root != "" {
		fs, err = securefs.MakeFsOnDiskSecure(g.root)
	} else {
		fs = filesys.MakeFsOnDisk()
	}
	if err != nil {
		return UnchangedAction, "", err
	}

	// Determine if there already is a Kustomization file at the root,
	// as this means we do not have to generate one.
	for _, kfilename := range konfig.RecognizedKustomizationFileNames() {
		if kpath := filepath.Join(dirPath, kfilename); fs.Exists(kpath) && !fs.IsDir(kpath) {
			return UnchangedAction, kpath, nil
		}
	}

	abs, err := filepath.Abs(dirPath)
	if err != nil {
		return UnchangedAction, "", err
	}

	files, err := scanManifests(fs, abs)
	if err != nil {
		return UnchangedAction, "", err
	}

	kfile := filepath.Join(dirPath, konfig.DefaultKustomizationFileName())
	f, err := fs.Create(kfile)
	if err != nil {
		return UnchangedAction, "", err
	}
	f.Close()

	kus := kustypes.Kustomization{
		TypeMeta: kustypes.TypeMeta{
			APIVersion: kustypes.KustomizationVersion,
			Kind:       kustypes.KustomizationKind,
		},
	}

	var resources []string
	for _, file := range files {
		resources = append(resources, strings.Replace(file, abs, ".", 1))
	}

	kus.Resources = resources
	kd, err := yaml.Marshal(kus)
	if err != nil {
		// delete the kustomization file
		errf := CleanDirectory(dirPath, CreatedAction)
		return UnchangedAction, "", fmt.Errorf("%v %v", err, errf)
	}

	return CreatedAction, kfile, os.WriteFile(kfile, kd, os.ModePerm)
}

// scanManifests walks through the given base path parsing all the files and
// collecting a list of all the yaml file paths which can be used as
// kustomization resources.
func scanManifests(fs filesys.FileSystem, base string) ([]string, error) {
	var paths []string
	pvd := provider.NewDefaultDepProvider()
	rf := pvd.GetResourceFactory()
	err := fs.Walk(base, func(path string, info os.FileInfo, err error) (walkErr error) {
		if err != nil {
			walkErr = err
			return
		}
		if path == base {
			return
		}
		if info.IsDir() {
			// If a sub-directory contains an existing kustomization file add the
			// directory as a resource and do not decend into it.
			for _, kfilename := range konfig.RecognizedKustomizationFileNames() {
				if kpath := filepath.Join(path, kfilename); fs.Exists(kpath) && !fs.IsDir(kpath) {
					paths = append(paths, path)
					return filepath.SkipDir
				}
			}
			return
		}

		extension := filepath.Ext(path)
		if extension != ".yaml" && extension != ".yml" {
			return
		}

		fContents, err := fs.ReadFile(path)
		if err != nil {
			walkErr = err
			return
		}

		// Kustomize YAML parser tends to panic in unpredicted ways due to
		// (accidental) invalid object data; recover when this happens to ensure
		// continuity of operations.
		defer func() {
			if r := recover(); r != nil {
				walkErr = fmt.Errorf("recovered from panic while parsing YAML file %s: %v", filepath.Base(path), r)
			}
		}()

		if _, err := rf.SliceFromBytes(fContents); err != nil {
			walkErr = fmt.Errorf("failed to decode Kubernetes YAML from %s: %w", path, err)
			return
		}
		paths = append(paths, path)
		return
	})
	return paths, err
}

func adaptSelector(selector *kustomize.Selector) (output *kustypes.Selector) {
	if selector != nil {
		output = &kustypes.Selector{}
		output.Gvk.Group = selector.Group
		output.Gvk.Kind = selector.Kind
		output.Gvk.Version = selector.Version
		output.Name = selector.Name
		output.Namespace = selector.Namespace
		output.LabelSelector = selector.LabelSelector
		output.AnnotationSelector = selector.AnnotationSelector
	}
	return
}

// buildMutex protects against kustomize concurrent map read/write panic
var kustomizeBuildMutex sync.Mutex

// Secure Build wraps krusty.MakeKustomizer with the following settings:
//   - secure on-disk FS denying operations outside root
//   - load files from outside the kustomization dir path
//     (but not outside root)
//   - disable plugins except for the builtin ones
func SecureBuild(root, dirPath string, allowRemoteBases bool) (res resmap.ResMap, err error) {
	var fs filesys.FileSystem

	// Create secure FS for root with or without remote base support
	if allowRemoteBases {
		fs, err = securefs.MakeFsOnDiskSecureBuild(root)
		if err != nil {
			return nil, err
		}
	} else {
		fs, err = securefs.MakeFsOnDiskSecure(root)
		if err != nil {
			return nil, err
		}
	}
	return Build(fs, dirPath)
}

// Build wraps krusty.MakeKustomizer with the following settings:
// - load files from outside the kustomization.yaml root
// - disable plugins except for the builtin ones
func Build(fs filesys.FileSystem, dirPath string) (res resmap.ResMap, err error) {
	// temporary workaround for concurrent map read and map write bug
	// https://github.com/kubernetes-sigs/kustomize/issues/3659
	kustomizeBuildMutex.Lock()
	defer kustomizeBuildMutex.Unlock()

	// Kustomize tends to panic in unpredicted ways due to (accidental)
	// invalid object data; recover when this happens to ensure continuity of
	// operations
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from kustomize build panic: %v", r)
		}
	}()

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	return k.Run(fs, dirPath)
}

// CleanDirectory removes the kustomization.yaml file from the given directory.
func CleanDirectory(dirPath string, action Action) error {
	// find original kustomization file
	var originalFile string
	for _, file := range konfig.RecognizedKustomizationFileNames() {
		originalKustomizationFile := filepath.Join(dirPath, file+".original")
		if _, err := os.Stat(originalKustomizationFile); err == nil {
			originalFile = originalKustomizationFile
			break
		}
	}

	// figure out file name
	kfile := filepath.Join(dirPath, konfig.DefaultKustomizationFileName())
	if originalFile != "" {
		kfile = strings.TrimSuffix(originalFile, ".original")
	}

	// restore old file if it exists
	if _, err := os.Stat(originalFile); err == nil {
		err := os.Rename(originalFile, kfile)
		if err != nil {
			return fmt.Errorf("failed to cleanup repository: %w", err)
		}
	}

	if action == CreatedAction {
		return os.Remove(kfile)
	}

	return nil
}

// copyFile copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist or else trucnated.
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}

	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}

	defer func() {
		errf := out.Close()
		if err == nil {
			err = errf
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return
	}

	return
}

func IsLocalRelativePath(path string) bool {
	// From: https://github.com/kubernetes-sigs/kustomize/blob/84bd402cc0662c5df3f109c4f80c22611243c5f9/api/internal/git/repospec.go#L231-L239
	// with "file://" removed/
	for _, p := range []string{
		// Order matters here.
		"git::", "gh:", "ssh://", "https://", "http://",
		"git@", "github.com:", "github.com/"} {
		if len(p) < len(path) && strings.ToLower(path[:len(p)]) == p {
			return false
		}
	}

	if filepath.IsAbs(path) || filepath.IsAbs(strings.TrimPrefix(strings.ToLower(path), "file://")) {
		return false
	}
	return true
}
