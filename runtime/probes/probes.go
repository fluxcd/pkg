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

package probes

import (
	"os"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

// SetupChecks configures simple default ready and health probes on the given mgr.
//
// The func can be used in the main.go file of your controller, after initialisation of the manager:
//
//		func main() {
//			mgr, err := ctrl.NewManager(cfg, ctrl.Options{})
//			if err != nil {
//				log.Error(err, "unable to start manager")
//				os.Exit(1)
//			}
//			probes.SetupChecks(mgr, log)
//	 }
func SetupChecks(mgr ctrl.Manager, log logr.Logger) {
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		log.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		log.Error(err, "unable to create health check")
		os.Exit(1)
	}
}
