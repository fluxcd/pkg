/*
Copyright 2020, 2022 The Flux authors

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
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"

	"golang.org/x/crypto/openpgp"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/testserver"
)

const (
	keyRingName = "TestUser"
)

// NewTempHelmServer returns a HTTP HelmServer with a newly created
// temp dir as repository docroot.
func NewTempHelmServer() (*HelmServer, error) {
	tmpDir, err := os.MkdirTemp("", "helm-test-")
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
	return os.WriteFile(f, d, 0644)
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
	return s.packageChart(path, version, "")
}

// PackageSignedChartWithVersion attempts to package the chart at the given path
// with the given version and sign it using an internally generated PGP keyring, to be served
// by the HelmServer. publicKeyPath is the path where the public key should be written to, which
// can be used to verify this chart. It returns an error in case of a packaging failure.
func (s *HelmServer) PackageSignedChartWithVersion(path, version, publicKeyPath string) error {
	return s.packageChart(path, version, publicKeyPath)
}

func (s *HelmServer) packageChart(path, version, publicKeyPath string) error {
	pkg := action.NewPackage()
	pkg.Destination = s.Root()
	pkg.Version = version
	if publicKeyPath != "" {
		randBytes := make([]byte, 16)
		rand.Read(randBytes)
		secretKeyPath := filepath.Join(s.Root(), "secret-"+hex.EncodeToString(randBytes)+".pgp")
		if err := generateKeyring(secretKeyPath, publicKeyPath); err != nil {
			return err
		}
		defer os.Remove(secretKeyPath)
		pkg.Keyring = secretKeyPath
		pkg.Key = keyRingName
		pkg.Sign = true
	}
	_, err := pkg.Run(path, nil)
	return err
}

func generateKeyring(privateKeyPath, publicKeyPath string) error {
	entity, err := openpgp.NewEntity(keyRingName, "", "", nil)
	if err != nil {
		return err
	}
	priv, err := os.Create(privateKeyPath)
	defer priv.Close()
	if err != nil {
		return err
	}
	pub, err := os.Create(publicKeyPath)
	defer pub.Close()
	if err != nil {
		return err
	}
	if err := entity.SerializePrivate(priv, nil); err != nil {
		return err
	}
	if err := entity.Serialize(pub); err != nil {
		return err
	}

	return nil
}
