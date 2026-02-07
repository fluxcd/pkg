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

// auth is a package for handling short-lived credentials.
// Flux APIs using this package will never pass in contents
// of a Secret specified directly in the API object under
// reconciliation. Instead, options to generate short-lived
// credentials on-the-fly shall be provided. The package
// supports caching of generated credentials to avoid
// rate-limiting by external services that are part of
// the credential generation process.
package auth
