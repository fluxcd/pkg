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
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
