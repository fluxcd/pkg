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

package masktoken

import (
	"fmt"
	"regexp"
)

// MaskTokenFromString redacts all matches for the given token from the provided string,
// replacing them with "*****".
// The token is expected to be a valid UTF-8 string.
// This can for example be used to remove sensitive information from error messages.
func MaskTokenFromString(log string, token string) (string, error) {
	if token == "" {
		return log, nil
	}

	re, err := regexp.Compile(fmt.Sprintf("%s*", regexp.QuoteMeta(token)))
	if err != nil {
		return "", err
	}

	redacted := re.ReplaceAllString(log, "*****")

	return redacted, nil
}
