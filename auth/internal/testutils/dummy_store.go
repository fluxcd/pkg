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

package testutils

import (
	"time"

	"github.com/fluxcd/pkg/auth"
)

type dummyCache struct {
	items map[string]interface{}
}

func NewDummyCache() auth.Store {
	return &dummyCache{
		items: make(map[string]interface{}),
	}
}

var _ auth.Store = &dummyCache{}

func (c *dummyCache) Set(key string, item interface{}, _ time.Duration) error {
	c.items[key] = item
	return nil
}

func (c *dummyCache) Get(key string) (interface{}, bool) {
	item, ok := c.items[key]
	return item, ok
}
