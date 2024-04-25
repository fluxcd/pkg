/*
Copyright 2024 The Flux authors

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

package login

import (
	"time"

	"github.com/fluxcd/pkg/cache"
	"github.com/google/go-containerregistry/pkg/authn"
)

func cacheObject[T authn.Authenticator](store cache.Expirable[cache.StoreObject[T]], auth T, key string, expiresAt *time.Time) {
	obj := cache.StoreObject[T]{
		Object: auth,
		Key:    key,
	}
	store.Set(obj)
	if expiresAt == nil || expiresAt.IsZero() {
		return
	}
	store.SetExpiration(obj, *expiresAt)
}

func getObjectFromCache[T authn.Authenticator](cache cache.Expirable[cache.StoreObject[T]], key string) (T, bool, error) {
	var res T
	val, exists, err := cache.GetByKey(key)
	if err != nil {
		return res, true, err
	}
	if exists {
		return val.Object, true, nil
	}
	return res, false, nil
}
