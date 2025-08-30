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

package meta

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHistoryUpsert(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() *History
		digest         string
		status         string
		expectedLength int
		expectedTotal  int64
	}{
		{
			name: "add new snapshot to empty history",
			setup: func() *History {
				h := &History{}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 1,
			expectedTotal:  1,
		},
		{
			name: "update existing snapshot",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 1,
			expectedTotal:  2,
		},
		{
			name: "update existing snapshot and move to front",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:xyz123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Minute)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Minute)),
						LastReconciledDuration: metav1.Duration{Duration: time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:xyz123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						LastReconciledDuration: metav1.Duration{Duration: time.Second},
						LastReconciledStatus:   "Failed",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:bcd123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						LastReconciledDuration: metav1.Duration{Duration: time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:bcd123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-2 * time.Minute)),
						LastReconciledDuration: metav1.Duration{Duration: time.Second},
						LastReconciledStatus:   "Failed",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Minute)),
						LastReconciledDuration: metav1.Duration{Duration: 15 * time.Second},
						LastReconciledStatus:   "Failed",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 5,
			expectedTotal:  2,
		},
		{
			name: "add new snapshot due to different status",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Failure",
			expectedLength: 2,
			expectedTotal:  1,
		},
		{
			name: "add new snapshot to existing history",
			setup: func() *History {
				h := &History{
					{
						Digest:                 "sha256:def456",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
				}
				return h
			},
			digest:         "sha256:abc123",
			status:         "Success",
			expectedLength: 2,
			expectedTotal:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.setup()
			metadata := map[string]string{"test": "value"}

			h.Upsert(tt.digest, time.Now(), 45*time.Second, tt.status, metadata)

			if len(*h) != tt.expectedLength {
				t.Errorf("expectedLength length %d, got %d", tt.expectedLength, len(*h))
			}

			// Verify latest entry matches the upserted snapshot
			if h.Latest().Digest != tt.digest && h.Latest().LastReconciledStatus != tt.status {
				t.Errorf("expectedLength first snapshot digest %s status %s, got %s %s",
					tt.digest, tt.status, h.Latest().Digest, h.Latest().LastReconciledStatus)
			}

			// Verify total reconciliations
			if tt.expectedTotal != h.Latest().TotalReconciliations {
				t.Errorf("expectedLength total reconciliations to be %d, got %d",
					tt.expectedTotal, h.Latest().TotalReconciliations)
			}

			// Verify metadata was set
			for _, snapshot := range *h {
				if snapshot.Digest == tt.digest && snapshot.LastReconciledStatus == tt.status {
					if snapshot.Metadata["test"] != "value" {
						t.Errorf("expectedLength metadata 'test'='value', got %v", snapshot.Metadata)
					}
				}
			}
		})
	}
}

func TestHistoryTruncate(t *testing.T) {
	h := &History{}
	baseTime := time.Now()

	// Add more than HistoryMaxSize snapshots to trigger truncation
	overflow := HistoryMaxSize + 2
	digests := make([]string, overflow)
	for i := 0; i < overflow; i++ {
		digests[i] = "sha256:digest" + string(rune('0'+i))
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		h.Upsert(digests[i], timestamp, 30*time.Second, "Success", nil)
	}

	// Should be truncated to HistoryMaxSize
	if len(*h) != HistoryMaxSize {
		t.Errorf("expected length %d after truncation, got %d", HistoryMaxSize, len(*h))
	}

	// Verify the history contains the most recent snapshots in reverse order
	for i := 0; i < HistoryMaxSize; i++ {
		expectedDigest := digests[overflow-(i+1)]
		if (*h)[i].Digest != expectedDigest {
			t.Errorf("expected digest at index %d to be %s, got %s", i, expectedDigest, (*h)[i].Digest)
		}
	}
}

func TestHistoryLatest(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() History
		expected *Snapshot
	}{
		{
			name: "empty history",
			setup: func() History {
				return History{}
			},
			expected: nil,
		},
		{
			name: "single snapshot",
			setup: func() History {
				return History{
					{
						Digest:                 "sha256:abc123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
				}
			},
			expected: &Snapshot{
				Digest: "sha256:abc123",
			},
		},
		{
			name: "multiple snapshots",
			setup: func() History {
				return History{
					{
						Digest:                 "sha256:new123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
					{
						Digest:                 "sha256:old123",
						FirstReconciled:        metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciled:         metav1.NewTime(time.Now().Add(-1 * time.Hour)),
						LastReconciledDuration: metav1.Duration{Duration: 30 * time.Second},
						LastReconciledStatus:   "Success",
						TotalReconciliations:   1,
					},
				}
			},
			expected: &Snapshot{
				Digest: "sha256:new123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := tt.setup()
			result := h.Latest()

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expectedLength nil, got %v", result)
				}
			} else {
				if result == nil {
					t.Errorf("expectedLength %v, got nil", tt.expected)
				} else if result.Digest != tt.expected.Digest {
					t.Errorf("expectedLength digest %s, got %s", tt.expected.Digest, result.Digest)
				}
			}
		})
	}
}
