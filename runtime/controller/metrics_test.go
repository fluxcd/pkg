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

package controller_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/runtime/conditions/testdata"
	"github.com/fluxcd/pkg/runtime/controller"
)

func TestMetrics_IsDelete(t *testing.T) {
	testFinalizers := []string{"finalizers.fluxcd.io", "finalizers.foo.bar"}
	timenow := metav1.NewTime(time.Now())

	tests := []struct {
		name            string
		finalizers      []string
		deleteTimestamp *metav1.Time
		ownedFinalizers []string
		want            bool
	}{
		{"equal finalizers, no delete timestamp", testFinalizers, nil, testFinalizers, false},
		{"partial finalizers, no delete timestamp", []string{"finalizers.fluxcd.io"}, nil, testFinalizers, false},
		{"unknown finalizers, no delete timestamp", []string{"foo"}, nil, testFinalizers, false},
		{"unknown finalizers, delete timestamp", []string{"foo"}, &timenow, testFinalizers, true},
		{"no finalizers, no delete timestamp", []string{}, nil, testFinalizers, false},
		{"no owned finalizers, no delete timestamp", []string{"foo"}, nil, nil, false},
		{"no finalizers, delete timestamp", []string{}, &timenow, testFinalizers, true},
		{"no finalizers, no delete timestamp", nil, nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			mgr, err := ctrl.NewManager(&rest.Config{}, ctrl.Options{})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(mgr).NotTo(BeNil())

			metrics := controller.NewMetrics(mgr, nil, tt.ownedFinalizers...)
			obj := &testdata.Fake{}
			obj.SetFinalizers(tt.finalizers)
			obj.SetDeletionTimestamp(tt.deleteTimestamp)
			g.Expect(metrics.IsDelete(obj)).To(Equal(tt.want))
		})
	}
}
