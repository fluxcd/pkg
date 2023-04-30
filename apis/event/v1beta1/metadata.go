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

package v1beta1

// These constants define the Event metadata keys used throughout Flux controllers.
const (
	// MetaRevisionKey is the key used to hold the source artifact revision.
	MetaRevisionKey string = "revision"
	// MetaChecksumKey is the key used to hold the source artifact checksum.
	// Deprecated: in favor of MetaDigestKey.
	MetaChecksumKey string = "checksum"
	// MetaDigestKey is the key used to hold the source artifact digest.
	MetaDigestKey string = "digest"
	// MetaTokenKey is the key used to hold an arbitrary token whose contents
	// are defined on a per-event-emitter basis for uniquely identifying the
	// contents of the event payload. For example, it could be the generation
	// of an object, or the hash of a set of configurations, or even a
	// base64-encoded set of configurations. This is useful for example for
	// rate limiting the events.
	MetaTokenKey string = "token"
	// MetaCommitStatusKey is the key used to signal a Git commit status event.
	MetaCommitStatusKey string = "commit_status"
	// MetaCommitStatusUpdateValue is the value of MetaCommitStatusKey
	// used to signal a Git commit status update.
	MetaCommitStatusUpdateValue string = "update"
)
