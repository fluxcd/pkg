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

package helmtestserver

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/openpgp"
	"helm.sh/helm/v3/pkg/downloader"
)

func TestPackageSignedChartWithVersion(t *testing.T) {
	server, err := NewTempHelmServer()
	defer os.RemoveAll(server.Root())
	if err != nil {
		t.Fatal(err)
	}
	publicKeyPath := filepath.Join(server.Root(), "pub.pgp")
	packagedChartPath := filepath.Join(server.Root(), "helmchart-0.1.0.tgz")
	if err := server.PackageSignedChartWithVersion("./testdata/helmchart", "0.1.0", publicKeyPath); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(packagedChartPath); err != nil {
		t.Fatal(err)
	}

	out, err := os.Open(publicKeyPath)
	defer out.Close()
	if err != nil {
		t.Fatal(err)
	}

	if _, err = openpgp.ReadKeyRing(out); err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(fmt.Sprintf("%s.prov", packagedChartPath)); err != nil {
		t.Fatal(err)
	}

	if _, err = downloader.VerifyChart(packagedChartPath, publicKeyPath); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateIndex(t *testing.T) {
	server, err := NewTempHelmServer()
	defer os.RemoveAll(server.Root())
	if err != nil {
		t.Fatal(err)
	}

	if err := server.PackageChartWithVersion("./testdata/helmchart", "0.1.0"); err != nil {
		t.Fatal(err)
	}

	if err := server.GenerateIndex(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(fmt.Sprintf("%s/%s", server.Root(), "index.yaml")); err != nil {
		t.Fatal(err)
	}
}
