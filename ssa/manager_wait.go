/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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

package ssa

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/cli-utils/pkg/kstatus/polling"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/aggregator"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/collector"
	"github.com/fluxcd/cli-utils/pkg/kstatus/polling/event"
	"github.com/fluxcd/cli-utils/pkg/kstatus/status"
	"github.com/fluxcd/cli-utils/pkg/object"

	"github.com/fluxcd/pkg/ssa/utils"
)

// WaitOptions contains options for wait requests.
type WaitOptions struct {
	// Interval defines how often to poll the cluster for the latest state of the resources.
	Interval time.Duration

	// Timeout defines after which interval should the engine give up on waiting for resources
	// to become ready.
	Timeout time.Duration

	// FailFast makes the Wait function return an error as soon as a resource reaches the failed state.
	FailFast bool
}

// DefaultWaitOptions returns the default wait options where the poll interval is set to
// five seconds and the timeout to one minute.
func DefaultWaitOptions() WaitOptions {
	return WaitOptions{
		Interval: 5 * time.Second,
		Timeout:  60 * time.Second,
	}
}

// Wait checks if the given set of objects has been fully reconciled.
func (m *ResourceManager) Wait(objects []*unstructured.Unstructured, opts WaitOptions) error {
	objectsMeta := object.UnstructuredSetToObjMetadataSet(objects)
	if len(objectsMeta) == 0 {
		return nil
	}

	return m.WaitForSet(objectsMeta, opts)
}

// WaitForSet checks if the given ObjMetadataSet has been fully reconciled.
func (m *ResourceManager) WaitForSet(set object.ObjMetadataSet, opts WaitOptions) error {
	return m.WaitForSetWithContext(context.Background(), set, opts)
}

// WaitForSetWithContext checks if the given ObjMetadataSet has been fully reconciled.
// The provided context can be used to cancel the operation.
func (m *ResourceManager) WaitForSetWithContext(ctx context.Context, set object.ObjMetadataSet, opts WaitOptions) error {
	statusCollector := collector.NewResourceStatusCollector(set)
	canceledInternally := false

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	pollingOpts := polling.PollOptions{
		PollInterval: opts.Interval,
	}
	eventsChan := m.poller.Poll(ctx, set, pollingOpts)

	lastStatus := make(map[object.ObjMetadata]*event.ResourceStatus)

	done := statusCollector.ListenWithObserver(eventsChan, collector.ObserverFunc(
		func(statusCollector *collector.ResourceStatusCollector, e event.Event) {
			var rss []*event.ResourceStatus
			var countFailed int
			for _, rs := range statusCollector.ResourceStatuses {
				if rs == nil {
					continue
				}
				// skip DeadlineExceeded errors because kstatus emits that error
				// for every resource it's monitoring even when only one of them
				// actually fails.
				if !errors.Is(rs.Error, context.DeadlineExceeded) {
					lastStatus[rs.Identifier] = rs
				}

				if rs.Status == status.FailedStatus {
					countFailed++
				}
				rss = append(rss, rs)
			}

			desired := status.CurrentStatus
			aggStatus := aggregator.AggregateStatus(rss, desired)
			if aggStatus == desired || (opts.FailFast && countFailed > 0) {
				canceledInternally = true
				cancel()
				return
			}
		}),
	)

	<-done

	// If the context was cancelled externally, return early.
	if !canceledInternally && errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}

	if statusCollector.Error != nil {
		return statusCollector.Error
	}

	var errs []string
	for id, rs := range statusCollector.ResourceStatuses {
		switch {
		case rs == nil || lastStatus[id] == nil:
			errs = append(errs, fmt.Sprintf("can't determine status for %s", utils.FmtObjMetadata(id)))
		case lastStatus[id].Status == status.FailedStatus,
			errors.Is(ctx.Err(), context.DeadlineExceeded) &&
				lastStatus[id].Status != status.CurrentStatus:
			var builder strings.Builder
			if utils.IsSuspended(lastStatus[id].Resource) {
				// skip suspended resources that are not in a failed state
				// as they are not expected to be reconciled and
				// their observed generation will always be behind
				continue
			}
			builder.WriteString(fmt.Sprintf("%s status: '%s'",
				utils.FmtObjMetadata(rs.Identifier), lastStatus[id].Status))
			if rs.Error != nil {
				builder.WriteString(fmt.Sprintf(": %s", rs.Error))
			}
			errs = append(errs, builder.String())
		}
	}

	if len(errs) > 0 {
		msg := "failed early due to stalled resources"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			msg = "timeout waiting for"
		}
		return fmt.Errorf("%s: [%s]", msg, strings.Join(errs, ", "))
	}

	return nil
}

// WaitForSetTermination waits for the termination of resources
// specified in the given ChangeSet within the given options.
// Only resources marked for deletion are considered.
func (m *ResourceManager) WaitForSetTermination(cs *ChangeSet, opts WaitOptions) error {
	if cs == nil || len(cs.Entries) == 0 {
		return nil
	}

	objects := make([]*unstructured.Unstructured, 0)

	// Filter out entries that are not marked for deletion.
	for _, entry := range cs.Entries {
		if entry.Action != DeletedAction {
			continue
		}

		gvk := schema.GroupVersionKind{
			Group:   entry.ObjMetadata.GroupKind.Group,
			Kind:    entry.ObjMetadata.GroupKind.Kind,
			Version: entry.GroupVersion,
		}

		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		obj.SetName(entry.ObjMetadata.Name)
		obj.SetNamespace(entry.ObjMetadata.Namespace)

		objects = append(objects, obj)
	}

	if len(objects) == 0 {
		return nil
	}

	return m.WaitForTermination(objects, opts)
}

// WaitForTermination waits for the given objects to be deleted from the cluster.
func (m *ResourceManager) WaitForTermination(objects []*unstructured.Unstructured, opts WaitOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	for _, obj := range objects {
		if err := wait.PollUntilContextCancel(ctx, opts.Interval, true, m.isDeleted(obj)); err != nil {
			return fmt.Errorf("%s termination timeout: %w", utils.FmtUnstructured(obj), err)
		}
	}
	return nil
}

func (m *ResourceManager) isDeleted(object *unstructured.Unstructured) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		obj := object.DeepCopy()
		err := m.client.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
}
