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
	"strings"
	"testing"
)

func Test_MaskTokenFromError(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		expectErr      bool
		originalErrStr string
		expectedErrStr string
	}{
		{
			name:           "no token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: "Cannot post to github",
			expectedErrStr: "Cannot post to github",
		},
		{
			name:           "empty token",
			token:          "",
			originalErrStr: "Cannot post to github",
			expectedErrStr: "Cannot post to github",
		},
		{
			name:           "exact token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: "Cannot post to github with token 8h0387hdyehbwwa45",
			expectedErrStr: "Cannot post to github with token *****",
		},
		{
			name:           "non-exact token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: `Cannot post to github with token 8h0387hdyehbwwa45\\n`,
			expectedErrStr: `Cannot post to github with token *****\\n`,
		},
		{
			name:           "extra text in front token",
			token:          "8h0387hdyehbwwa45",
			originalErrStr: `Cannot post to github with token metoo8h0387hdyehbwwa45\\n`,
			expectedErrStr: `Cannot post to github with token metoo*****\\n`,
		},
		{
			name:           "extra text in front token",
			token:          "8h0387hdyehbwwa45踙",
			originalErrStr: `Cannot post to github with token metoo8h0387hdyehbwwa45踙\\n`,
			expectedErrStr: `Cannot post to github with token metoo*****\\n`,
		},
		{
			name:           "return error on invalid UTF-8 string",
			token:          "\x18\xd0\xfa\xab\xb2\x93\xbb;\xc0l\xf4\xdc",
			originalErrStr: `Cannot post to github with token \x18\xd0\xfa\xab\xb2\x93\xbb;\xc0l\xf4\xdc\\n`,
			expectedErrStr: ``,
			expectErr:      true,
		},
		{
			name:           "unescaped token",
			token:          "8h0387hdyehbwwa45\\",
			originalErrStr: `Cannot post to github with token metoo8h0387hdyehbwwa45\\\n`,
			expectedErrStr: `Cannot post to github with token metoo*****n`,
		},
		{
			name:           "invalid chars",
			token:          "8h0387hdyehbwwa45(?!\\/)",
			originalErrStr: `Cannot post to github`,
			expectedErrStr: `Cannot post to github`,
		},
	}

	for _, tt := range tests {
		returnedStr, err := MaskTokenFromString(tt.originalErrStr, tt.token)
		if tt.expectErr && err == nil {
			t.Fatalf("expected error for token: %s", tt.token)
		}

		if !tt.expectErr && err != nil {
			t.Fatalf("returned unexpected error: %s", err)
		}

		if !strings.Contains(returnedStr, tt.expectedErrStr) {
			t.Errorf("expected returned string '%s' to contain '%s'",
				returnedStr, tt.expectedErrStr)
		}
	}

}
