/*
Copyright 2026 The Flux authors

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
	"os"
	"path/filepath"
	"strings"
	"sync"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"google.golang.org/protobuf/proto"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/kustomize/api/konfig"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/openapi/kubernetesapi"
	"sigs.k8s.io/yaml"
)

const (
	openAPIPathField          = "path"
	mergedOpenAPIPathFileName = ".flux-openapi-merged.json"
)

var (
	kubernetesOpenAPISchemaOnce sync.Once
	kubernetesOpenAPISchema     *spec.Swagger
	kubernetesOpenAPISchemaErr  error
)

func mergeOpenAPIPathWithBuiltins(fs filesys.FileSystem, dirPath string) error {
	names := recognizedKustomizationFileNames()

	return fs.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if _, ok := names[filepath.Base(path)]; !ok {
			return nil
		}
		return mergeKustomizationOpenAPIPathWithBuiltins(fs, path)
	})
}

func recognizedKustomizationFileNames() map[string]struct{} {
	names := make(map[string]struct{}, len(konfig.RecognizedKustomizationFileNames()))
	for _, name := range konfig.RecognizedKustomizationFileNames() {
		names[name] = struct{}{}
	}
	return names
}

func mergeKustomizationOpenAPIPathWithBuiltins(fs filesys.FileSystem, kfile string) error {
	data, err := fs.ReadFile(kfile)
	if err != nil {
		return fmt.Errorf("failed to read kustomization file %s: %w", kfile, err)
	}

	var kus kustypes.Kustomization
	if err := yaml.Unmarshal(data, &kus); err != nil {
		return fmt.Errorf("failed to parse kustomization file %s: %w", kfile, err)
	}

	openAPIPath := strings.TrimSpace(kus.OpenAPI[openAPIPathField])
	if openAPIPath == "" {
		return nil
	}

	schemaPath := openAPIPath
	if !filepath.IsAbs(schemaPath) {
		schemaPath = filepath.Join(filepath.Dir(kfile), schemaPath)
	}

	userSchema, err := readOpenAPISchema(fs, schemaPath)
	if err != nil {
		return err
	}

	mergedSchema, err := mergeOpenAPISchemaWithBuiltins(userSchema)
	if err != nil {
		return err
	}

	mergedData, err := json.Marshal(mergedSchema)
	if err != nil {
		return fmt.Errorf("failed to encode merged OpenAPI schema for %s: %w", kfile, err)
	}
	mergedData = append(mergedData, '\n')

	mergedPath := filepath.Join(filepath.Dir(kfile), mergedOpenAPIPathFileName)
	if err := fs.WriteFile(mergedPath, mergedData); err != nil {
		return fmt.Errorf("failed to write merged OpenAPI schema %s: %w", mergedPath, err)
	}

	kus.OpenAPI[openAPIPathField] = mergedOpenAPIPathFileName
	updatedData, err := yaml.Marshal(kus)
	if err != nil {
		return fmt.Errorf("failed to encode kustomization file %s: %w", kfile, err)
	}
	if err := fs.WriteFile(kfile, updatedData); err != nil {
		return fmt.Errorf("failed to write kustomization file %s: %w", kfile, err)
	}

	return nil
}

func readOpenAPISchema(fs filesys.FileSystem, path string) (*spec.Swagger, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAPI schema %s: %w", path, err)
	}

	data, err = yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert OpenAPI schema %s to JSON: %w", path, err)
	}

	var swagger spec.Swagger
	if err := swagger.UnmarshalJSON(data); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI schema %s: %w", path, err)
	}

	return &swagger, nil
}

func mergeOpenAPISchemaWithBuiltins(userSchema *spec.Swagger) (*spec.Swagger, error) {
	mergedSchema, err := loadKubernetesOpenAPISchema()
	if err != nil {
		return nil, err
	}

	if userSchema == nil {
		return mergedSchema, nil
	}

	if mergedSchema.Definitions == nil {
		mergedSchema.Definitions = spec.Definitions{}
	}
	for name, definition := range userSchema.Definitions {
		mergedSchema.Definitions[name] = definition
	}

	if userSchema.Paths != nil {
		if mergedSchema.Paths == nil {
			mergedSchema.Paths = &spec.Paths{}
		}
		if mergedSchema.Paths.Paths == nil {
			mergedSchema.Paths.Paths = map[string]spec.PathItem{}
		}
		for path, item := range userSchema.Paths.Paths {
			mergedSchema.Paths.Paths[path] = item
		}
		if len(userSchema.Paths.Extensions) > 0 {
			if mergedSchema.Paths.Extensions == nil {
				mergedSchema.Paths.Extensions = map[string]interface{}{}
			}
			for name, value := range userSchema.Paths.Extensions {
				mergedSchema.Paths.Extensions[name] = value
			}
		}
	}

	if len(userSchema.Extensions) > 0 {
		if mergedSchema.Extensions == nil {
			mergedSchema.Extensions = map[string]interface{}{}
		}
		for name, value := range userSchema.Extensions {
			mergedSchema.Extensions[name] = value
		}
	}

	return mergedSchema, nil
}

func loadKubernetesOpenAPISchema() (*spec.Swagger, error) {
	kubernetesOpenAPISchemaOnce.Do(func() {
		kubernetesOpenAPISchema, kubernetesOpenAPISchemaErr = decodeKubernetesOpenAPISchema()
	})
	if kubernetesOpenAPISchemaErr != nil {
		return nil, kubernetesOpenAPISchemaErr
	}
	return cloneOpenAPISchema(kubernetesOpenAPISchema)
}

func decodeKubernetesOpenAPISchema() (*spec.Swagger, error) {
	version := kubernetesapi.DefaultOpenAPI
	assetName := filepath.Join("kubernetesapi", strings.ReplaceAll(version, ".", "_"), "swagger.pb")
	data := kubernetesapi.OpenAPIMustAsset[version](assetName)

	doc := &openapi_v2.Document{}
	if err := proto.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("failed to decode embedded Kubernetes OpenAPI schema: %w", err)
	}

	var swagger spec.Swagger
	ok, err := swagger.FromGnostic(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert embedded Kubernetes OpenAPI schema: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("failed to convert embedded Kubernetes OpenAPI schema")
	}

	return &swagger, nil
}

func cloneOpenAPISchema(swagger *spec.Swagger) (*spec.Swagger, error) {
	data, err := json.Marshal(swagger)
	if err != nil {
		return nil, fmt.Errorf("failed to clone embedded Kubernetes OpenAPI schema: %w", err)
	}

	var clone spec.Swagger
	if err := clone.UnmarshalJSON(data); err != nil {
		return nil, fmt.Errorf("failed to clone embedded Kubernetes OpenAPI schema: %w", err)
	}
	return &clone, nil
}
