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

package jitter

import (
	"math/rand"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestJitteredRequeueInterval(t *testing.T) {
	r := rand.New(rand.NewSource(int64(12345)))
	p := 0.2
	SetGlobalIntervalJitter(p, r)

	tests := []struct {
		name           string
		res            ctrl.Result
		expectModified bool
	}{
		{res: ctrl.Result{Requeue: true}, expectModified: false},
		{res: ctrl.Result{RequeueAfter: 0}, expectModified: false},
		{res: ctrl.Result{RequeueAfter: 10 * time.Second}, expectModified: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if tt.expectModified {
				lowerBound := float64(tt.res.RequeueAfter) * (1 - p)
				upperBound := float64(tt.res.RequeueAfter) * (1 + p)

				for i := 0; i < 100; i++ {
					res := JitteredRequeueInterval(tt.res)

					g.Expect(res.RequeueAfter).To(BeNumerically(">=", lowerBound))
					g.Expect(res.RequeueAfter).To(BeNumerically("<=", upperBound))
					g.Expect(res.RequeueAfter).ToNot(Equal(tt.res.RequeueAfter))
				}
			} else {
				g.Expect(JitteredRequeueInterval(tt.res)).To(Equal(tt.res))
			}
		})
	}
}

func TestJitteredIntervalDuration(t *testing.T) {
	g := NewWithT(t)

	r := rand.New(rand.NewSource(int64(12345)))
	p := 0.5
	SetGlobalIntervalJitter(p, r)

	interval := 10 * time.Second
	lowerBound := float64(interval) * (1 - p)
	upperBound := float64(interval) * (1 + p)

	for i := 0; i < 100; i++ {
		d := JitteredIntervalDuration(interval)

		g.Expect(d).To(BeNumerically(">=", lowerBound))
		g.Expect(d).To(BeNumerically("<=", upperBound))
		g.Expect(d).ToNot(Equal(interval))
	}
}

func TestIntervalOptions_BindFlags(t *testing.T) {
	g := NewWithT(t)

	interval := &IntervalOptions{}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	interval.BindFlags(fs)

	g.Expect(interval.Percentage).To(Equal(uint8(defaultIntervalJitterPercentage)))
}

func TestIntervalOptions_BindFlagsWithDefault(t *testing.T) {
	g := NewWithT(t)

	t.Run("with default fallback", func(t *testing.T) {
		interval := &IntervalOptions{}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		interval.BindFlagsWithDefault(fs, -1)

		g.Expect(interval.Percentage).To(Equal(uint8(defaultIntervalJitterPercentage)))
	})

	t.Run("with custom default", func(t *testing.T) {
		interval := &IntervalOptions{}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		interval.BindFlagsWithDefault(fs, 50)

		g.Expect(interval.Percentage).To(Equal(uint8(50)))
	})

	t.Run("with flag override", func(t *testing.T) {
		interval := &IntervalOptions{}

		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		interval.BindFlagsWithDefault(fs, 0)

		err := fs.Set(flagIntervalJitter, "25")
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(interval.Percentage).To(Equal(uint8(25)))
	})
}

func TestIntervalOptions_SetGlobalJitter(t *testing.T) {
	t.Run("invalid percentage >=100", func(t *testing.T) {
		g := NewWithT(t)

		interval := &IntervalOptions{Percentage: uint8(100)}
		err := interval.SetGlobalJitter(nil)
		g.Expect(err).To(MatchError(errInvalidIntervalJitter))
	})
}
