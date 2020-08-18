/*
Copyright 2020 The Flux CD contributors.

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

package helmtestserver

import (
	"io/ioutil"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/testserver"
)

// NewTempHelmServer returns a HTTP HelmServer with a newly created
// temp dir as repository docroot.
func NewTempHelmServer() (*HelmServer, error) {
	tmpDir, err := ioutil.TempDir("", "helm-test-")
	if err != nil {
		return nil, err
	}
	server := testserver.NewHTTPServer(tmpDir)
	helm := &HelmServer{server}
	return helm, nil
}

// HelmServer is a Helm repository server for testing purposes.
// It can serve repository indexes and charts over HTTP/S.
type HelmServer struct {
	*testserver.HTTPServer
}

// GenerateIndex (re)generates the repository index.
func (s *HelmServer) GenerateIndex() error {
	index, err := repo.IndexDirectory(s.HTTPServer.Root(), s.HTTPServer.URL())
	if err != nil {
		return err
	}
	d, err := yaml.Marshal(index)
	if err != nil {
		return err
	}
	f := filepath.Join(s.HTTPServer.Root(), "index.yaml")
	return ioutil.WriteFile(f, d, 0644)
}

// PackageChart attempts to package the chart at the given path, to be served
// by the HelmServer. It returns an error in case of a packaging failure.
func (s *HelmServer) PackageChart(path string) error {
	return s.PackageChartWithVersion(path, "")
}

// PackageChartWithVersion attempts to package the chart at the given path
// with the given version, to be served by the HelmServer. It returns an
// error in case of a packaging failure.
func (s *HelmServer) PackageChartWithVersion(path, version string) error {
	pkg := action.NewPackage()
	pkg.Destination = s.HTTPServer.Root()
	pkg.Version = version
	_, err := pkg.Run(path, nil)
	return err
}
