/*
Copyright 2024 The Flux authors

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
	"bufio"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/envsubst"
)

const (
	// varsubRegex is the regular expression used to validate
	// the var names before substitution
	varsubRegex   = "^[_[:alpha:]][_[:alpha:][:digit:]]*$"
	DisabledValue = "disabled"
)

const (
	postBuildField          = "postBuild"
	substituteFromField     = "substituteFrom"
	substituteField         = "substitute"
	substituteAnnotationKey = "kustomize.toolkit.fluxcd.io/substitute"
)

// SubstituteOptions defines the options for the variable substitutions operation.
type SubstituteOptions struct {
	DryRun bool
	Strict bool
}

type SubstituteOption func(a *SubstituteOptions)

// SubstituteWithDryRun sets the dryRun option.
// When dryRun is true, the substitution process will not attempt to talk to the cluster.
func SubstituteWithDryRun(dryRun bool) SubstituteOption {
	return func(a *SubstituteOptions) {
		a.DryRun = dryRun
	}
}

// SubstituteWithStrict sets the strict option.
// When strict is true, the substitution process will fail if a var without a
// default value is declared in files but is missing from the input vars.
func SubstituteWithStrict(strict bool) SubstituteOption {
	return func(a *SubstituteOptions) {
		a.Strict = strict
	}
}

// SubstituteVariables replaces the vars with their values in the specified resource.
// If a resource is labeled or annotated with
// 'kustomize.toolkit.fluxcd.io/substitute: disabled' the substitution is skipped.
func SubstituteVariables(
	ctx context.Context,
	kubeClient client.Client,
	kustomization unstructured.Unstructured,
	res *resource.Resource,
	opts ...SubstituteOption) (*resource.Resource, error) {
	var options SubstituteOptions
	for _, o := range opts {
		o(&options)
	}

	resData, err := res.AsYAML()
	if err != nil {
		return nil, err
	}

	if res.GetLabels()[substituteAnnotationKey] == DisabledValue || res.GetAnnotations()[substituteAnnotationKey] == DisabledValue {
		return nil, nil
	}

	// load vars from ConfigMaps and Secrets data keys
	// In dryRun mode this step is skipped. This might in different kind of errors.
	// But if the user is using dryRun, he/she should know what he/she is doing, and we should comply.
	var vars map[string]string
	if !options.DryRun {
		vars, err = LoadVariables(ctx, kubeClient, kustomization)
		if err != nil {
			return nil, err
		}
	}

	// load in-line vars (overrides the ones from resources)
	substitute, ok, err := unstructured.NestedStringMap(kustomization.Object, specField, postBuildField, substituteField)
	if err != nil {
		return nil, err
	}
	if ok {
		if vars == nil {
			vars = make(map[string]string)
		}
		for k, v := range substitute {
			vars[k] = strings.ReplaceAll(v, "\n", "")
		}
	}

	// run bash variable substitutions
	if len(vars) > 0 {
		jsonData, err := varSubstitution(resData, vars, options.Strict)
		if err != nil {
			return nil, fmt.Errorf("envsubst error: %w", err)
		}
		err = res.UnmarshalJSON(jsonData)
		if err != nil {
			return nil, fmt.Errorf("UnmarshalJSON: %w", err)
		}
	}

	return res, nil
}

// LoadVariables reads the in-line variables set in the Flux Kustomization and merges them with
// the vars referred in ConfigMaps and Secrets data keys.
func LoadVariables(ctx context.Context, kubeClient client.Client, kustomization unstructured.Unstructured) (map[string]string, error) {
	vars := make(map[string]string)
	substituteFrom, err := getSubstituteFrom(kustomization)
	if err != nil {
		return nil, fmt.Errorf("unable to get subsituteFrom: %w", err)
	}

	for _, reference := range substituteFrom {
		namespacedName := types.NamespacedName{Namespace: kustomization.GetNamespace(), Name: reference.Name}
		switch reference.Kind {
		case "ConfigMap":
			cm := &corev1.ConfigMap{}
			if err := kubeClient.Get(ctx, namespacedName, cm); err != nil {
				if reference.Optional && apierrors.IsNotFound(err) {
					continue
				}
				return nil, fmt.Errorf("substitute from 'ConfigMap/%s' error: %w", reference.Name, err)
			}
			for k, v := range cm.Data {
				vars[k] = strings.ReplaceAll(v, "\n", "")
			}
		case "Secret":
			secret := &corev1.Secret{}
			if err := kubeClient.Get(ctx, namespacedName, secret); err != nil {
				if reference.Optional && apierrors.IsNotFound(err) {
					continue
				}
				return nil, fmt.Errorf("substitute from 'Secret/%s' error: %w", reference.Name, err)
			}
			for k, v := range secret.Data {
				vars[k] = strings.ReplaceAll(string(v), "\n", "")
			}
		}
	}

	return vars, nil
}

func varSubstitution(data []byte, vars map[string]string, strict bool) ([]byte, error) {
	r, _ := regexp.Compile(varsubRegex)
	for v := range vars {
		if !r.MatchString(v) {
			return nil, fmt.Errorf("'%s' var name is invalid, must match '%s'", v, varsubRegex)
		}
	}

	output, err := envsubst.Eval(string(data), func(s string) (string, bool) {
		if strict {
			v, exists := vars[s]
			return v, exists
		}
		return vars[s], true
	})
	if err != nil {
		return nil, fmt.Errorf("variable substitution failed: %w", err)
	}

	jsonData, err := yaml.YAMLToJSON([]byte(output))
	if err != nil {
		return nil, fmt.Errorf("YAMLToJSON: %w", err)
	}

	return jsonData, nil
}

func getSubstituteFrom(kustomization unstructured.Unstructured) ([]SubstituteReference, error) {
	substituteFrom, ok, err := unstructured.NestedSlice(kustomization.Object, specField, postBuildField, substituteFromField)
	if err != nil {
		return nil, err
	}

	var resultErr error
	if ok {
		res := make([]SubstituteReference, 0, len(substituteFrom))
		for k, s := range substituteFrom {
			sub, ok := s.(map[string]interface{})
			if !ok {
				err := fmt.Errorf("unable to convert patch %d to map[string]interface{}", k)
				resultErr = errors.Join(resultErr, err)
			}
			var substitute SubstituteReference
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(sub, &substitute)
			if err != nil {
				resultErr = errors.Join(resultErr, err)
			}
			res = append(res, substitute)
		}
		return res, resultErr
	}

	return nil, resultErr
}

// SubstituteEnvVariables performs variable substitution on multi-document YAML
// input, skipping resources annotated or labeled with
// kustomize.toolkit.fluxcd.io/substitute: disabled. The mapping function
// resolves variable names to values; it is called for each ${var} reference
// in non-disabled documents.
func SubstituteEnvVariables(data string, mapping func(string) (string, bool)) (string, error) {
	chunks, seps := splitYAMLDocuments(data)

	var b strings.Builder
	for i, chunk := range chunks {
		if i > 0 {
			b.WriteString(seps[i-1])
		}
		if isSubstituteDisabled(chunk) {
			b.WriteString(chunk)
			continue
		}
		out, err := envsubst.Eval(chunk, mapping)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}

// isSubstituteDisabled reports whether a raw YAML document carries the
// kustomize.toolkit.fluxcd.io/substitute: disabled annotation or label.
func isSubstituteDisabled(doc string) bool {
	if strings.TrimSpace(doc) == "" {
		return false
	}
	var m struct {
		Metadata struct {
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := yaml.Unmarshal([]byte(doc), &m); err != nil {
		return false
	}
	return m.Metadata.Labels[substituteAnnotationKey] == DisabledValue ||
		m.Metadata.Annotations[substituteAnnotationKey] == DisabledValue
}

// splitYAMLDocuments splits multi-document YAML into content chunks and the
// separator strings between them. A separator is a line that is exactly "---"
// with optional trailing whitespace. The returned slices satisfy
// len(seps) == len(chunks)-1.
func splitYAMLDocuments(data string) (chunks []string, seps []string) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	var cur strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if isDocSeparator(line) {
			chunks = append(chunks, cur.String())
			cur.Reset()
			seps = append(seps, line+"\n")
		} else {
			cur.WriteString(line)
			cur.WriteByte('\n')
		}
	}
	trailing := cur.String()
	if len(trailing) > 0 && !strings.HasSuffix(data, "\n") {
		trailing = strings.TrimSuffix(trailing, "\n")
	}
	chunks = append(chunks, trailing)
	return chunks, seps
}

// isDocSeparator reports whether line is a YAML document separator,
// i.e. exactly "---" optionally followed by spaces or tabs.
func isDocSeparator(line string) bool {
	if !strings.HasPrefix(line, "---") {
		return false
	}
	for _, r := range line[3:] {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}
