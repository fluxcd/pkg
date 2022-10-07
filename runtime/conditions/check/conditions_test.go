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

package check

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParse(t *testing.T) {
	g := NewWithT(t)
	input := []byte(`
negativePolarity:
- foo
- bar
positivePolarity:
- aaa
- bbb
`)
	c, err := ParseConditions(input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(c.NegativePolarity)).To(Equal(2))
	g.Expect(len(c.PositivePolarity)).To(Equal(2))
}

func TestHighestNegativePriorityCondition(t *testing.T) {
	tests := []struct {
		name             string
		input            []metav1.Condition
		negativePolarity []string
		wantType         string
		wantErr          bool
	}{
		{
			name: "one negative polarity",
			input: []metav1.Condition{
				{Type: "failureA", Status: "True", Reason: "fooA", Message: "barA"},
				{Type: "failureB", Status: "True", Reason: "fooB", Message: "barB"},
				{Type: "failureC", Status: "True", Reason: "fooC", Message: "barC"},
			},
			negativePolarity: []string{"failureB"},
			wantType:         "failureB",
		},
		{
			name: "multiple negative polarities",
			input: []metav1.Condition{
				{Type: "failureA", Status: "True", Reason: "fooA", Message: "barA"},
				{Type: "failureB", Status: "True", Reason: "fooB", Message: "barB"},
				{Type: "failureC", Status: "True", Reason: "fooC", Message: "barC"},
			},
			negativePolarity: []string{"failureC", "failureA", "failureB"},
			wantType:         "failureC",
		},
		{
			name: "no negative polarities",
			input: []metav1.Condition{
				{Type: "failureA", Status: "True", Reason: "fooA", Message: "barA"},
				{Type: "failureB", Status: "True", Reason: "fooB", Message: "barB"},
				{Type: "failureC", Status: "True", Reason: "fooC", Message: "barC"},
			},
			wantErr: true,
		},
		{
			name: "no matching condition",
			input: []metav1.Condition{
				{Type: "failureA", Status: "True", Reason: "fooA", Message: "barA"},
				{Type: "failureB", Status: "True", Reason: "fooB", Message: "barB"},
				{Type: "failureC", Status: "True", Reason: "fooC", Message: "barC"},
			},
			negativePolarity: []string{"failureX", "failureY", "failureZ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			conditions := &Conditions{
				NegativePolarity: tt.negativePolarity,
			}

			result, err := HighestNegativePriorityCondition(conditions, tt.input)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.wantType == "" {
				g.Expect(result).To(BeNil())
			} else {
				g.Expect(result.Type).To(Equal(tt.wantType))
			}
		})
	}
}
