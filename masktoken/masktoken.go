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

func MaskTokenFromString(log string, token string) string {
	if token == "" {
		return log
	}

	re, compileErr := regexp.Compile(fmt.Sprintf("%s*", regexp.QuoteMeta(token)))
	if compileErr != nil {
		newErrStr := fmt.Sprintf("error redacting token from string: %s", compileErr)
		return newErrStr
	}

	redacted := re.ReplaceAllString(log, "*****")

	return redacted
}
