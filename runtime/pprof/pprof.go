/*
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

package pprof

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var endpoints = map[string]http.Handler{
	"/debug/pprof/":        http.HandlerFunc(pprof.Index),
	"/debug/pprof/cmdline": http.HandlerFunc(pprof.Cmdline),
	"/debug/pprof/profile": http.HandlerFunc(pprof.Profile),
	"/debug/pprof/symbol":  http.HandlerFunc(pprof.Symbol),
	"/debug/pprof/trace":   http.HandlerFunc(pprof.Trace),
}

func SetupHandlers(mgr ctrl.Manager, setupLog logr.Logger) {
	for p, h := range endpoints {
		if err := mgr.AddMetricsExtraHandler(p, h); err != nil {
			setupLog.Error(err, "unable to add pprof handler")
		}
	}
}
