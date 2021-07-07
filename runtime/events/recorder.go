/*
Copyright 2020 The Flux authors

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

package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Recorder posts events to the webhook address.
type Recorder struct {
	// URL address of the events endpoint.
	Webhook string

	// Name of the controller that emits events.
	ReportingController string

	// Retryable HTTP client.
	Client *retryablehttp.Client
}

// NewRecorder creates an event Recorder with default settings.
// The recorder performs automatic retries for connection errors and 500-range response codes.
func NewRecorder(webhook, reportingController string) (*Recorder, error) {
	if _, err := url.Parse(webhook); err != nil {
		return nil, err
	}

	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Timeout = 5 * time.Second
	httpClient.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy
	httpClient.Logger = nil

	return &Recorder{
		Webhook:             webhook,
		ReportingController: reportingController,
		Client:              httpClient,
	}, nil
}

// EventInfof records an event with information severity.
func (r *Recorder) EventInfof(
	object corev1.ObjectReference,
	metadata map[string]string,
	reason string, messageFmt string, args ...interface{}) error {
	return r.Eventf(object, metadata, EventSeverityInfo, reason, messageFmt, args...)
}

// EventErrorf records an event with error severity.
func (r *Recorder) EventErrorf(
	object corev1.ObjectReference,
	metadata map[string]string,
	reason string, messageFmt string, args ...interface{}) error {
	return r.Eventf(object, metadata, EventSeverityError, reason, messageFmt, args...)
}

// Eventf constructs an event from the given information and performs a HTTP POST to the webhook address.
func (r *Recorder) Eventf(
	object corev1.ObjectReference,
	metadata map[string]string,
	severity, reason string,
	messageFmt string, args ...interface{}) error {
	if r.Client == nil {
		return fmt.Errorf("retryable HTTP client has not been initialized")
	}

	message := fmt.Sprintf(messageFmt, args...)

	if object.Kind == "" {
		return fmt.Errorf("failed to get object kind")
	}

	if object.Name == "" {
		return fmt.Errorf("failed to get object name")
	}

	if object.Namespace == "" {
		return fmt.Errorf("failed to get object namespace")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	event := Event{
		InvolvedObject:      object,
		Severity:            severity,
		Timestamp:           metav1.Now(),
		Message:             message,
		Reason:              reason,
		Metadata:            metadata,
		ReportingController: r.ReportingController,
		ReportingInstance:   hostname,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal object into json, error: %w", err)
	}

	// avoid retrying rate limited requests
	if res, _ := r.Client.HTTPClient.Post(r.Webhook, "application/json", bytes.NewReader(body)); res != nil &&
		(res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusAccepted) {
		return nil
	}

	if _, err := r.Client.Post(r.Webhook, "application/json", body); err != nil {
		return err
	}

	return nil
}
