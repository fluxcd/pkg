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
	"runtime"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HTTPPrefixPProf is the prefix appended to all Endpoints.
const HTTPPrefixPProf = "/debug/pprof"

// Endpoints defines the debugging endpoints that are added by SetupHandlers.
var Endpoints = map[string]http.Handler{
	HTTPPrefixPProf + "/":             http.HandlerFunc(pprof.Index),
	HTTPPrefixPProf + "/cmdline":      http.HandlerFunc(pprof.Cmdline),
	HTTPPrefixPProf + "/profile":      http.HandlerFunc(pprof.Profile),
	HTTPPrefixPProf + "/symbol":       http.HandlerFunc(pprof.Symbol),
	HTTPPrefixPProf + "/trace":        http.HandlerFunc(pprof.Trace),
	HTTPPrefixPProf + "/heap":         pprof.Handler("heap"),
	HTTPPrefixPProf + "/goroutine":    pprof.Handler("goroutine"),
	HTTPPrefixPProf + "/threadcreate": pprof.Handler("threadcreate"),
	HTTPPrefixPProf + "/block":        pprof.Handler("block"),
	HTTPPrefixPProf + "/mutex":        pprof.Handler("mutex"),
}

// SetupHandlers registers the pprof endpoints on the metrics server of the given mgr.
//
// The func can be used in the main.go file of your controller, after initialisation of the manager:
//
//	func main() {
//		mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
//		if err != nil {
//			log.Error(err, "unable to start manager")
//			os.Exit(1)
//		}
//		pprof.SetupHandlers(mgr, log)
//	}
func SetupHandlers(mgr ctrl.Manager, log logr.Logger) {
	// Only set the fraction if there is no existing setting
	if runtime.SetMutexProfileFraction(-1) == 0 {
		// Default to report 1 out of 5 mutex events, on average
		runtime.SetMutexProfileFraction(5)
	}

	for p, h := range Endpoints {
		if err := mgr.AddMetricsExtraHandler(p, h); err != nil {
			log.Error(err, "unable to add pprof handler")
		}
	}
}
