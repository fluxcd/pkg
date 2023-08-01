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
	"fmt"
	"math/rand"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestNoJitter(t *testing.T) {
	g := NewWithT(t)

	g.Expect(NoJitter(10 * time.Second)).To(Equal(10 * time.Second))
	g.Expect(NoJitter(0)).To(Equal(0 * time.Second))
	g.Expect(NoJitter(-10 * time.Second)).To(Equal(-10 * time.Second))
}

func TestPercent(t *testing.T) {
	r := rand.New(rand.NewSource(int64(12345)))

	tests := []struct {
		p        float64
		duration time.Duration
	}{
		{p: 0.1, duration: 100 * time.Millisecond},
		{p: 0, duration: 100 * time.Millisecond},
		{p: 1, duration: 100 * time.Millisecond},
		{p: -1, duration: 100 * time.Millisecond},
		{p: 2, duration: 100 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("p=%v, duration=%v", tt.p, tt.duration), func(t *testing.T) {
			g := NewWithT(t)

			fn := Percent(tt.p, r)

			if tt.p > 0 && tt.p < 1 {
				for i := 0; i < 100; i++ {
					lowerBound := float64(tt.duration) * (1 - tt.p)
					upperBound := float64(tt.duration) * (1 + tt.p)

					d := fn(tt.duration)
					g.Expect(d).To(BeNumerically(">=", lowerBound))
					g.Expect(d).To(BeNumerically("<=", upperBound))
					g.Expect(d).ToNot(Equal(tt.duration))
				}
			} else {
				g.Expect(fn(tt.duration)).To(Equal(tt.duration))
			}
		})
	}
}
