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

package cache

import (
	"context"
	"time"

	"github.com/spf13/pflag"
)

// TokenMaxDuration is the maximum duration that a token can have in the
// TokenCache. This is used to cap the duration of tokens to avoid storing
// tokens that are valid for too long.
const TokenMaxDuration = time.Hour

// Token is an interface that represents an access token that can be used
// to authenticate with a cloud provider. The only common method is to get the
// duration of the token, because different providers may have different ways to
// represent the token. For example, Azure and GCP use an opaque string token,
// while AWS uses the pair of access key id and secret access key. Consumers of
// this token should know what type to cast this interface to.
type Token interface {
	// GetDuration returns the duration for which the token is valid relative to
	// approximately time.Now(). This is used to determine when the token should
	// be refreshed.
	GetDuration() time.Duration
}

// TokenCache is a thread-safe cache specialized in storing and retrieving
// access tokens. It uses an LRU cache as the underlying storage and takes
// care of expiring tokens in a pessimistic way by storing both a timestamp
// with a monotonic clock (the Go default) and an absolute timestamp created
// from the Unix timestamp of when the token was created. The token is
// considered expired when either timestamps are older than the current time.
// This strategy ensures that expired tokens aren't kept in the cache for
// longer than their expiration time. Also, tokens expire on 80% of their
// lifetime, which is the same strategy used by kubelet for rotating
// ServiceAccount tokens.
type TokenCache struct {
	cache       *LRU[*tokenItem]
	maxDuration time.Duration
}

// TokenFlags contains the CLI flags that can be used to configure the TokenCache.
type TokenFlags struct {
	MaxSize     int
	MaxDuration time.Duration
}

type tokenItem struct {
	token Token
	mono  time.Time
	unix  time.Time
}

// NewTokenCache returns a new TokenCache with the given capacity.
func NewTokenCache(capacity int, opts ...Options) (*TokenCache, error) {
	o := storeOptions{maxDuration: TokenMaxDuration}
	o.apply(opts...)

	if o.maxDuration > TokenMaxDuration {
		o.maxDuration = TokenMaxDuration
	}

	cache, err := NewLRU[*tokenItem](capacity, opts...)
	if err != nil {
		return nil, err
	}

	return &TokenCache{cache, o.maxDuration}, nil
}

// GetOrSet returns the token for the given key if present and not expired, or
// calls the newToken function to get a new token and stores it in the cache.
// The operation is thread-safe and atomic. The boolean return value indicates
// whether the token was retrieved from the cache.
func (c *TokenCache) GetOrSet(ctx context.Context,
	key string,
	newToken func(context.Context) (Token, error),
	opts ...Options,
) (Token, bool, error) {

	condition := func(token *tokenItem) bool {
		return !token.expired()
	}

	fetch := func(ctx context.Context) (*tokenItem, error) {
		token, err := newToken(ctx)
		if err != nil {
			return nil, err
		}
		return c.newItem(token), nil
	}

	opts = append(opts, func(so *storeOptions) error {
		so.debugKey = "token"
		so.debugValueFunc = func(v any) any {
			return map[string]any{
				"duration": v.(*tokenItem).token.GetDuration().String(),
			}
		}
		return nil
	})

	item, ok, err := c.cache.GetIfOrSet(ctx, key, condition, fetch, opts...)
	if err != nil {
		return nil, false, err
	}
	return item.token, ok, nil
}

// DeleteEventsForObject deletes all cache events (cache_miss and cache_hit) for
// the associated object being deleted, given its kind, name and namespace.
func (c *TokenCache) DeleteEventsForObject(kind, name, namespace, operation string) {
	if c == nil {
		return
	}
	for _, eventType := range allEventTypes {
		c.cache.DeleteCacheEvent(eventType, kind, name, namespace, operation)
	}
}

func (c *TokenCache) newItem(token Token) *tokenItem {
	// Kubelet rotates ServiceAccount tokens when 80% of their lifetime has
	// passed, so we'll use the same threshold to consider tokens expired.
	//
	// Ref: https://github.com/kubernetes/kubernetes/blob/4032177faf21ae2f99a2012634167def2376b370/pkg/kubelet/token/token_manager.go#L172-L174
	d := (token.GetDuration() * 8) / 10

	if m := c.maxDuration; d > m {
		d = m
	}

	mono := time.Now().Add(d)
	unix := time.Unix(mono.Unix(), 0)

	return &tokenItem{
		token: token,
		mono:  mono,
		unix:  unix,
	}
}

func (ti *tokenItem) expired() bool {
	now := time.Now()
	return !ti.mono.After(now) || !ti.unix.After(now)
}

func (f *TokenFlags) BindFlags(fs *pflag.FlagSet, defaultMaxSize int) {
	fs.IntVar(&f.MaxSize, "token-cache-max-size", defaultMaxSize,
		"The maximum size of the cache in number of tokens.")
	fs.DurationVar(&f.MaxDuration, "token-cache-max-duration", TokenMaxDuration,
		"The maximum duration a token is cached.")
}
