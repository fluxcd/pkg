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
	MetaChecksumKey string = "checksum"
	// MetaCommitStatusKey is the key used to signal a Git commit status event.
	MetaCommitStatusKey string = "commit_status"
	// MetaCommitStatusUpdateValue is the value of MetaCommitStatusKey
	// used to signal a Git commit status update.
	MetaCommitStatusUpdateValue string = "update"
)
