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

package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/cmd/internal"
)

var pkgReleaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release the github.com/fluxcd/pkg repository Go modules",
	RunE:  runRelease,
}

func init() {
	pkgCmd.AddCommand(pkgReleaseCmd)
}

func runRelease(cmd *cobra.Command, args []string) error {
	ctx := ctrl.SetupSignalHandler()

	res, err := internal.ComputeModuleBumps(ctx)
	if err != nil {
		return fmt.Errorf("failed to compute module bumps: %w", err)
	}
	if res.MustBumpInternalModules() {
		res.PrintBumps()
		return errors.New("modules need to be bumped, please run 'make prep' first and open a pull request")
	}
	if !res.MustPushTags() {
		return nil
	}
	res.PrintTags()

	// Prompt for confirmation to push the tags.
	fmt.Println("\nConfirm pushing tags above to Git repository? (Y/n, only uppercase Y will confirm)")
	var response string
	fmt.Scanln(&response)
	if response != "Y" {
		fmt.Println("Aborting changes.")
		return nil
	}

	// Push the tags to the remote repository.
	if err := res.PushTags(ctx); err != nil {
		return fmt.Errorf("failed to push tags: %w", err)
	}

	return nil
}
