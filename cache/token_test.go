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
	"fmt"
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
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()

	tc, err := cache.NewTokenCache(1)
	g.Expect(err).NotTo(HaveOccurred())

	token, retrieved, err := tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: 2 * time.Second}, nil
	})
	g.Expect(token).To(Equal(&testToken{duration: 2 * time.Second}))
	g.Expect(retrieved).To(BeFalse())
	g.Expect(err).To(BeNil())

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) { return nil, nil })

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&testToken{duration: 2 * time.Second}))
	g.Expect(retrieved).To(BeTrue())

	time.Sleep(2 * time.Second)

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: time.Hour}, nil
	})
	g.Expect(token).To(Equal(&testToken{duration: time.Hour}))
	g.Expect(retrieved).To(BeFalse())
	g.Expect(err).To(BeNil())
}

func TestTokenCache_80PercentLifetime(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()

	tc, err := cache.NewTokenCache(1)
	g.Expect(err).NotTo(HaveOccurred())

	token, retrieved, err := tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: 5 * time.Second}, nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&testToken{duration: 5 * time.Second}))
	g.Expect(retrieved).To(BeFalse())

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) { return nil, nil })

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&testToken{duration: 5 * time.Second}))
	g.Expect(retrieved).To(BeTrue())

	time.Sleep(4 * time.Second)

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: time.Hour}, nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&testToken{duration: time.Hour}))
	g.Expect(retrieved).To(BeFalse())
}

func TestTokenCache_MaxDuration(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()

	tc, err := cache.NewTokenCache(1, cache.WithMaxDuration(time.Second))
	g.Expect(err).NotTo(HaveOccurred())

	token, retrieved, err := tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: time.Hour}, nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&testToken{duration: time.Hour}))
	g.Expect(retrieved).To(BeFalse())

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) { return nil, nil })

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&testToken{duration: time.Hour}))
	g.Expect(retrieved).To(BeTrue())

	time.Sleep(2 * time.Second)

	token, retrieved, err = tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return &testToken{duration: 10 * time.Millisecond}, nil
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&testToken{duration: 10 * time.Millisecond}))
	g.Expect(retrieved).To(BeFalse())
}

func TestTokenCache_GetOrSet_Error(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)

	ctx := context.Background()

	tc, err := cache.NewTokenCache(1)
	g.Expect(err).NotTo(HaveOccurred())

	token, retrieved, err := tc.GetOrSet(ctx, "test", func(context.Context) (cache.Token, error) {
		return nil, fmt.Errorf("failed")
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError("failed"))
	g.Expect(token).To(BeNil())
	g.Expect(retrieved).To(BeFalse())
}
