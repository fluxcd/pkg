/*
Copyright 2023 The Flux authors

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

package controller

import (
	"sync"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fluxcd/pkg/apis/meta"
)

const (
	flagWatchWatchAllNamespaces   = "watch-all-namespaces"
	flagWatchLabelSelector        = "watch-label-selector"
	flagWatchConfigsLabelSelector = "watch-configs-label-selector"
)

// WatchOptions defines the configurable options for reconciler resources watcher.
type WatchOptions struct {
	// AllNamespaces defines the watch filter at namespace level.
	// If set to false, the reconciler will only watch the runtime namespace for resource changes.
	AllNamespaces bool

	// LabelSelector defines the watch filter based on matching label expressions.
	// When set, the reconciler will only watch for changes of those resources with matching labels.
	// Docs: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#list-and-watch-filtering.
	LabelSelector string

	// ConfigsLabelSelector defines the watch filter for ConfigMaps and Secrets based on matching label expressions.
	// When set, the reconciler will only watch for changes of those resources with matching labels.
	// Docs: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#list-and-watch-filtering.
	ConfigsLabelSelector string
}

// BindFlags will parse the given pflag.FlagSet for the controller and
// set the WatchOptions accordingly.
func (o *WatchOptions) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.AllNamespaces, flagWatchWatchAllNamespaces, true,
		"Watch for resources in all namespaces, if set to false it will only watch the runtime namespace.")
	fs.StringVar(&o.LabelSelector, flagWatchLabelSelector, "",
		"Watch for resources with matching labels e.g. 'sharding.fluxcd.io/shard=shard1'.")
	fs.StringVar(&o.ConfigsLabelSelector, flagWatchConfigsLabelSelector, meta.LabelKeyWatch+"="+meta.LabelValueWatchEnabled,
		"Watch for ConfigMaps and Secrets with matching labels.")
}

// GetWatchLabelSelector parses the label selector option from WatchOptions
// and returns an error if the expression is invalid.
func GetWatchLabelSelector(opts WatchOptions) (*metav1.LabelSelector, error) {
	if opts.LabelSelector == "" {
		return nil, nil
	}

	return metav1.ParseToLabelSelector(opts.LabelSelector)
}

// GetWatchSelector parses the label selector option from WatchOptions and returns the label selector.
// If the WatchOptions contain no selectors, then a match everything is returned.
func GetWatchSelector(opts WatchOptions) (labels.Selector, error) {
	ls, err := GetWatchLabelSelector(opts)
	if err != nil {
		return nil, err
	}

	if ls == nil {
		return labels.Everything(), nil
	}

	return metav1.LabelSelectorAsSelector(ls)
}

// GetWatchConfigsPredicate parses the label selector option from WatchOptions
// and returns the controller-runtime predicate ready for setting up the watch.
func GetWatchConfigsPredicate(opts WatchOptions) (predicate.Predicate, error) {
	selector := labels.Everything()

	if opts.ConfigsLabelSelector != "" {
		var err error
		selector, err = labels.Parse(opts.ConfigsLabelSelector)
		if err != nil {
			return nil, err
		}
	}

	return predicate.NewPredicateFuncs(func(o client.Object) bool {
		return selector.Matches(labels.Set(o.GetLabels()))
	}), nil
}

// WatchedObjects keeps track of which objects are being watched and by which watchers.
type WatchedObjects struct {
	watches map[WatchedObject]map[WatcherObject]struct{}
	mu      sync.RWMutex
}

// WatchedObject represents a WatchedObject object, e.g. a ConfigMap or Secret.
type WatchedObject = client.ObjectKey

// WatcherObject represents an object that watches other objects, e.g. a Flux object.
type WatcherObject = client.ObjectKey

// NewWatchedObjects returns a new WatchedObjects instance and a corresponding predicate.Predicate.
// The predicate will only allow events for objects that are currently being watched. The WatchedObjects
// instance can be used to add or remove objects from the watch list.
func NewWatchedObjects() (*WatchedObjects, predicate.Predicate) {
	w := &WatchedObjects{
		watches: make(map[WatchedObject]map[WatcherObject]struct{}),
	}
	return w, predicate.NewPredicateFuncs(w.isWatched)
}

// isWatched checks if an object is currently being watched.
func (w *WatchedObjects) isWatched(obj client.Object) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, exists := w.watches[client.ObjectKeyFromObject(obj)]
	return exists
}

// ReconcileRequests returns the list of watchers for a given watched object.
func (w *WatchedObjects) ReconcileRequests(obj client.Object) []reconcile.Request {
	w.mu.RLock()
	defer w.mu.RUnlock()
	watchers := w.watches[client.ObjectKeyFromObject(obj)]
	result := make([]reconcile.Request, 0, len(watchers))
	for watcher := range watchers {
		result = append(result, reconcile.Request{NamespacedName: watcher})
	}
	return result
}

// Watch adds an object to the watch list.
func (w *WatchedObjects) Watch(obj, watcher client.Object) {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := client.ObjectKeyFromObject(obj)
	if w.watches[key] == nil {
		w.watches[key] = make(map[WatcherObject]struct{})
	}
	w.watches[key][client.ObjectKeyFromObject(watcher)] = struct{}{}
}

// Unwatch removes an object from the watch list.
func (w *WatchedObjects) Unwatch(obj, watcher client.Object) {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := client.ObjectKeyFromObject(obj)
	if watchers, exists := w.watches[key]; exists {
		delete(watchers, client.ObjectKeyFromObject(watcher))
		if len(watchers) == 0 {
			delete(w.watches, key)
		}
	}
}

// RemoveWatcher removes a watcher from all watched objects.
func (w *WatchedObjects) RemoveWatcher(watcher client.Object) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for obj, watchers := range w.watches {
		delete(watchers, client.ObjectKeyFromObject(watcher))
		if len(watchers) == 0 {
			delete(w.watches, obj)
		}
	}
}
