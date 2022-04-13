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

package controller

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/spf13/pflag"
)

func Test_RateLimiterOptions_BindFlags(t *testing.T) {
	tests := []struct {
		name                  string
		commandLine           []string
		expectedMinRetryDelay time.Duration
		expectedMaxRetryDelay time.Duration
	}{
		{
			name:                  "empty flags gets default values",
			commandLine:           []string{""},
			expectedMinRetryDelay: 750 * time.Millisecond,
			expectedMaxRetryDelay: 15 * time.Minute,
		},
		{
			name:                  "min retry delay only",
			commandLine:           []string{"--min-retry-delay=1s"},
			expectedMinRetryDelay: 1 * time.Second,
			expectedMaxRetryDelay: 15 * time.Minute,
		},
		{
			name:                  "max retry delay only",
			commandLine:           []string{"--max-retry-delay=5h"},
			expectedMinRetryDelay: 750 * time.Millisecond,
			expectedMaxRetryDelay: 5 * time.Hour,
		},
		{
			name: "min and max retry set",
			commandLine: []string{
				"--min-retry-delay=30s",
				"--max-retry-delay=24h",
			},
			expectedMinRetryDelay: 30 * time.Second,
			expectedMaxRetryDelay: 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			opts := RateLimiterOptions{}
			opts.BindFlags(f)

			err := f.Parse(tt.commandLine)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(opts.MaxRetryDelay).To(Equal(tt.expectedMaxRetryDelay))
			g.Expect(opts.MinRetryDelay).To(Equal(tt.expectedMinRetryDelay))
		})
	}
}
