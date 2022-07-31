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

package autologin

import "github.com/spf13/pflag"

const (
	flagAWSForECR   = "aws-autologin-for-ecr"
	flagAzureForAKS = "azure-autologin-for-acr"
	flagGCPForGCR   = "gcp-autologin-for-gcr"
)

// Options contains the auto-login configuration for cloud providers.
//
// The struct can be used in the main.go file of a controller by binding it to
// the main flag set, and then utilizing the configured options later:
//
//  func main() {
//		var (
//			// Other controller specific configuration variables.
// 			autologinOptions autologin.Options
//	 	)
//
//		// Bind the options to the main flag set, and parse it.
//		autologinOptions.BindFlags(flag.CommandLine)
// 		flag.Parse()
//  }
type Options struct {
	// AWSForECR enables AWS auto-login for ECR.
	AWSForECR bool
	// AzureForAKS enables Azure auto-login for AKS.
	AzureForAKS bool
	// GCPForGCR enables GCP auto-login for GCR.
	GCPForGCR bool
}

// BindFlags will parse the given pflag.FlagSet for auto-login option flags and
// set the Options accordingly.
func (o *Options) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.AWSForECR, flagAWSForECR, false,
		"(AWS) Attempt to get credentials for images in Elastic Container Registry, when no secret is referenced")
	fs.BoolVar(&o.AzureForAKS, flagAzureForAKS, false,
		"(Azure) Attempt to get credentials for images in Azure Container Registry, when no secret is referenced")
	fs.BoolVar(&o.GCPForGCR, flagGCPForGCR, false,
		"(GCP) Attempt to get credentials for images in Google Container Registry, when no secret is referenced")
}
