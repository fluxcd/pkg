/*
Copyright 2025 The Flux authors

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

package cache_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/cache"
)

type testToken struct {
	duration time.Duration
}

func (t *testToken) GetDuration() time.Duration {
	return t.duration
}

func TestTokenCache_Lifecycle(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	tc := cache.NewTokenCache(1)

	token, retrieved, err := tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: 2 * time.Second}, nil
	})
	g.Expect(token).To(Equal(&testToken{duration: 2 * time.Second}))
	g.Expect(retrieved).To(BeFalse())
	g.Expect(err).To(BeNil())

	time.Sleep(4 * time.Second)

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: 100 * time.Second}, nil
	})
	g.Expect(token).To(Equal(&testToken{duration: 100 * time.Second}))
	g.Expect(retrieved).To(BeFalse())
	g.Expect(err).To(BeNil())

	time.Sleep(2 * time.Second)

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) { return nil, nil })
	g.Expect(token).To(Equal(&testToken{duration: 100 * time.Second}))
	g.Expect(retrieved).To(BeTrue())
	g.Expect(err).To(BeNil())
}
