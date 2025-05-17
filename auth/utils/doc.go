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

// authutils contains small utility functions without much logic
// wrapping the major APIs of the core auth package for ease of use
// in the controllers. These functions also import the provider
// packages to wrap switch-case choice of provider implementations.
// Because of that, these functions cannot be placed in the core
// package as they would cause a cyclic dependency given that the
// provider packages also import the core package.
package authutils
