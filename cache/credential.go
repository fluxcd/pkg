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

// CredentialMaxDuration is the maximum duration that a credential can have in
// the CredentialCache. This is used to cap the duration of credentials to avoid
// storing credentials that are valid for too long.
const CredentialMaxDuration = time.Hour

// Credential is an interface that represents a credential that can be used to
// authenticate with a remote service. The only common method is to get the
// duration of the credential, because different credentials may have different
// representations. For example, Azure and GCP use an opaque string access token,
// while AWS uses the pair of access key id and secret access key. Consumers of
// this interface should know what type to cast it to.
type Credential interface {
	// GetDuration returns the duration for which the credential is valid
	// relative to approximately time.Now(). This is used to determine when the
	// credential should be refreshed.
	GetDuration() time.Duration
}

// CredentialCache is a thread-safe cache specialized in storing and retrieving
// credentials. It uses an LRU cache as the underlying storage and takes care of
// expiring credentials in a pessimistic way by storing both a timestamp with a
// monotonic clock (the Go default) and an absolute timestamp created from the
// Unix timestamp of when the credential was created. The credential is
// considered expired when either timestamp is older than the current time. This
// strategy ensures that expired credentials aren't kept in the cache for longer
// than their expiration time. Also, credentials expire on 80% of their lifetime,
// which is the same strategy used by kubelet for rotating ServiceAccount tokens.
type CredentialCache struct {
	cache       *LRU[*credentialItem]
	maxDuration time.Duration
}

// CredentialFlags contains the CLI flags that can be used to configure the CredentialCache.
type CredentialFlags struct {
	MaxSize     int
	MaxDuration time.Duration
}

type credentialItem struct {
	credential Credential
	mono       time.Time
	unix       time.Time
}

// NewCredentialCache returns a new CredentialCache with the given capacity.
func NewCredentialCache(capacity int, opts ...Options) (*CredentialCache, error) {
	o := storeOptions{maxDuration: CredentialMaxDuration}
	o.apply(opts...)

	if o.maxDuration > CredentialMaxDuration {
		o.maxDuration = CredentialMaxDuration
	}

	cache, err := NewLRU[*credentialItem](capacity, opts...)
	if err != nil {
		return nil, err
	}

	return &CredentialCache{cache, o.maxDuration}, nil
}

// GetOrSet returns the credential for the given key if present and not expired,
// or calls the newCredential function to get a new credential and stores it in
// the cache. The operation is thread-safe and atomic. The boolean return value
// indicates whether the credential was retrieved from the cache.
func (c *CredentialCache) GetOrSet(ctx context.Context,
	key string,
	newCredential func(context.Context) (Credential, error),
	opts ...Options,
) (Credential, bool, error) {

	condition := func(credential *credentialItem) bool {
		return !credential.expired()
	}

	fetch := func(ctx context.Context) (*credentialItem, error) {
		credential, err := newCredential(ctx)
		if err != nil {
			return nil, err
		}
		return c.newItem(credential), nil
	}

	opts = append(opts, func(so *storeOptions) error {
		so.debugKey = "credential"
		so.debugValueFunc = func(v any) any {
			return map[string]any{
				"duration": v.(*credentialItem).credential.GetDuration().String(),
			}
		}
		return nil
	})

	item, ok, err := c.cache.GetIfOrSet(ctx, key, condition, fetch, opts...)
	if err != nil {
		return nil, false, err
	}
	return item.credential, ok, nil
}

// DeleteEventsForObject deletes all cache events (cache_miss and cache_hit) for
// the associated object being deleted, given its kind, name and namespace.
func (c *CredentialCache) DeleteEventsForObject(kind, name, namespace, operation string) {
	if c == nil {
		return
	}
	for _, eventType := range allEventTypes {
		c.cache.DeleteCacheEvent(eventType, kind, name, namespace, operation)
	}
}

func (c *CredentialCache) newItem(credential Credential) *credentialItem {
	// Kubelet rotates ServiceAccount tokens when 80% of their lifetime has
	// passed, so we'll use the same threshold to consider credentials expired.
	//
	// Ref: https://github.com/kubernetes/kubernetes/blob/4032177faf21ae2f99a2012634167def2376b370/pkg/kubelet/token/token_manager.go#L172-L174
	d := (credential.GetDuration() * 8) / 10

	if m := c.maxDuration; d > m {
		d = m
	}

	mono := time.Now().Add(d)
	unix := time.Unix(mono.Unix(), 0)

	return &credentialItem{
		credential: credential,
		mono:       mono,
		unix:       unix,
	}
}

func (ci *credentialItem) expired() bool {
	now := time.Now()
	return !ci.mono.After(now) || !ci.unix.After(now)
}

func (f *CredentialFlags) BindFlags(fs *pflag.FlagSet, defaultMaxSize int) {
	fs.IntVar(&f.MaxSize, "token-cache-max-size", defaultMaxSize,
		"The maximum size of the cache in number of credentials.")
	fs.DurationVar(&f.MaxDuration, "token-cache-max-duration", CredentialMaxDuration,
		"The maximum duration a credential is cached.")
}
