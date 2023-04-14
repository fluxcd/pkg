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

package kustomize_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	kubeClient client.Client
)

func TestMain(m *testing.M) {
	testEnv := &envtest.Environment{}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}

	httpClient, err := rest.HTTPClientFor(cfg)
	if err != nil {
		panic(err)
	}
	restMapper, err := apiutil.NewDynamicRESTMapper(cfg, httpClient)
	if err != nil {
		panic(err)
	}

	kubeClient, err = client.New(cfg, client.Options{
		Mapper: restMapper,
	})
	if err != nil {
		panic(err)
	}

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}

func createObjectFile(kubeClient client.Client, objectFile string) error {
	buf, err := os.ReadFile(objectFile)
	if err != nil {
		return fmt.Errorf("Error reading file '%s': %v", objectFile, err)
	}

	clientObjects, err := readYamlObjects(strings.NewReader(string(buf)))
	if err != nil {
		return fmt.Errorf("Error decoding yaml file '%s': %v", objectFile, err)
	}
	err = createObjects(kubeClient, clientObjects)
	if err != nil {
		return fmt.Errorf("Error creating test objects: '%v'", err)
	}

	return nil
}

func readYamlObjects(rdr io.Reader) ([]unstructured.Unstructured, error) {
	objects := []unstructured.Unstructured{}
	reader := k8syaml.NewYAMLReader(bufio.NewReader(rdr))
	for {
		doc, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		unstructuredObj := &unstructured.Unstructured{}
		decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewBuffer(doc), len(doc))
		err = decoder.Decode(unstructuredObj)
		if err != nil {
			return nil, err
		}
		objects = append(objects, *unstructuredObj)
	}
	return objects, nil
}

func createObjects(kubeClient client.Client, objects []unstructured.Unstructured) error {
	for _, obj := range objects {
		createObj := obj.DeepCopy()
		err := kubeClient.Create(context.Background(), createObj)
		if err != nil {
			return err
		}
	}
	return nil
}
