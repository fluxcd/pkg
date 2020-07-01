/*
Copyright 2020 The Flux CD contributors.

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

package recorder

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EventRecorder posts events to the webhook address.
type EventRecorder struct {
	// URL address of the events endpoint.
	Webhook string

	// Name of the controller that emits events.
	ReportingController string

	// Retryable HTTP client
	Client *retryablehttp.Client
}

// NewEventRecorder creates an EventRecorder with default settings.
// The recorder performs automatic retries for connection errors and 500-range response code.
func NewEventRecorder(webhook, reportingController string) (*EventRecorder, error) {
	if _, err := url.Parse(webhook); err != nil {
		return nil, err
	}

	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Timeout = 5 * time.Second
	httpClient.Logger = nil

	return &EventRecorder{
		Webhook:             webhook,
		ReportingController: reportingController,
		Client:              httpClient,
	}, nil
}

// EventInfof records an event with information severity.
func (er *EventRecorder) EventInfof(
	object corev1.ObjectReference,
	metadata map[string]string,
	reason string, messageFmt string, args ...interface{}) error {
	return er.Eventf(object, metadata, EventSeverityInfo, reason, messageFmt, args...)
}

// EventErrorf records an event with error severity.
func (er *EventRecorder) EventErrorf(
	object corev1.ObjectReference,
	metadata map[string]string,
	reason string, messageFmt string, args ...interface{}) error {
	return er.Eventf(object, metadata, EventSeverityError, reason, messageFmt, args...)
}

// Eventf constructs an event from the given information
// and performs an HTTP POST to the webhook address.
func (er *EventRecorder) Eventf(
	object corev1.ObjectReference,
	metadata map[string]string,
	severity, reason string,
	messageFmt string, args ...interface{}) error {
	if er.Client == nil {
		return fmt.Errorf("retryable HTTP client has not been initilised")
	}

	message := fmt.Sprintf(messageFmt, args...)

	if object.Kind == "" {
		return fmt.Errorf("faild to get object kind")
	}

	if object.Name == "" {
		return fmt.Errorf("faild to get object name")
	}

	if object.Namespace == "" {
		return fmt.Errorf("faild to get object namespace")
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
		ReportingController: er.ReportingController,
		ReportingInstance:   hostname,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("faild to marshal object into json, error: %w", err)
	}

	if _, err := er.Client.Post(er.Webhook, "application/json", body); err != nil {
		return err
	}

	return nil
}
