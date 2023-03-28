/*
Copyright 2023 The Flux authors

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

package controller

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_WatchOptions_BindFlags(t *testing.T) {
	objects := []client.Object{
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "t0",
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "t1",
				Labels: map[string]string{
					"sharding.fluxcd.io/shard": "shard1",
				},
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "t2",
				Labels: map[string]string{
					"sharding.fluxcd.io/shard": "shard2",
				},
			},
		},
	}

	tests := []struct {
		name          string
		commandLine   []string
		objects       []client.Object
		expectedMatch []string
	}{
		{
			name:          "empty flag selects objects",
			commandLine:   []string{""},
			objects:       objects,
			expectedMatch: []string{"t0", "t1", "t2"},
		},
		{
			name:          "flag selects objects by label key val",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard=shard1"},
			objects:       objects,
			expectedMatch: []string{"t1"},
		},
		{
			name:          "flag selects objects by label exclusion expression",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard, sharding.fluxcd.io/shard notin (shard1)"},
			objects:       objects,
			expectedMatch: []string{"t2"},
		},
		{
			name:          "flag selects objects by label inclusion expression",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard in (shard1, shard2)"},
			objects:       objects,
			expectedMatch: []string{"t1", "t2"},
		},
		{
			name:          "flag selects objects with no matching labels",
			commandLine:   []string{"--watch-label-selector=sharding.fluxcd.io/shard notin (shard1, shard2)"},
			objects:       objects,
			expectedMatch: []string{"t0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			opts := WatchOptions{}
			opts.BindFlags(f)

			err := f.Parse(tt.commandLine)
			g.Expect(err).NotTo(HaveOccurred())

			sel, err := GetWatchSelector(opts)
			g.Expect(err).NotTo(HaveOccurred())

			for _, object := range tt.objects {
				if sel.Matches(labels.Set(object.GetLabels())) {
					var found bool
					for _, match := range tt.expectedMatch {
						if object.GetName() == match {
							found = true
						}
					}
					g.Expect(found).To(BeTrue())
				} else {
					var found bool
					for _, match := range tt.expectedMatch {
						if object.GetName() == match {
							found = true
						}
					}
					g.Expect(found).ToNot(BeTrue())
				}
			}
		})
	}
}
