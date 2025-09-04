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

// Package artifact is an SDK for managing Flux artifacts.
//
// This package provides comprehensive functionality for packaging artifacts, storing,
// serving, and processing in Flux source-controller. It includes:
//
// Configuration Management (config pkg):
//   - Flag binding with environment variable support for all artifact options
//   - Storage path, server address, and advertised address configuration
//   - Artifact retention policies (TTL and record count limits)
//   - Configurable digest algorithms for artifact integrity verification
//
// Artifact File Server (server pkg):
//   - Designed to serve artifacts over HTTP with proper content handling
//   - HTTP file server with graceful shutdown support compatible with controller-runtime
//
// Digest Computation (digest pkg):
//   - Multi-algorithm digest support (SHA1, SHA256, SHA512, BLAKE3)
//   - MultiDigester for computing multiple checksums simultaneously
//   - Algorithm validation and availability checking
//
// Storage Management (storage pkg):
//   - Complete artifact lifecycle management (create, verify, archive, cleanup)
//   - Atomic file operations with temporary file handling
//   - Tarball creation and extraction with filtering support
//   - Garbage collection based on TTL and retention policies
//   - File locking and symlink management
//   - Secure path handling to prevent directory traversal attacks
//   - Integration with Flux meta.Artifact types and OCI utilities
//
// The SDK standardizes artifact handling in Flux source-controller and provides
// a foundation for building 3rd party controllers that manage ExternalArtifacts.
package artifact
