/*
Copyright 2026 The Flux authors

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
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/fluxcd/pkg/cmd/internal"
)

var bumpCmd = &cobra.Command{
	Use:   "bump",
	Short: "Bump fluxcd/pkg dependencies in the current repository's go.mod",
	RunE:  runBump,
}

var bumpCmdFlags struct {
	preReleasePkg bool
}

func init() {
	rootCmd.AddCommand(bumpCmd)

	bumpCmd.Flags().BoolVar(&bumpCmdFlags.preReleasePkg, "pre-release-pkg", false,
		"Temporary flag for Flux 2.8: use the flux/v2.8.x pkg branch for main branches "+
			"because the pkg release branch was cut before the Flux distribution release. "+
			"Remove this flag once Flux 2.8.0 is released.")
}

func runBump(cmd *cobra.Command, args []string) error {
	ctx := setupSignalHandler()

	res, err := internal.BumpDeps(ctx, ".", bumpCmdFlags.preReleasePkg)
	if err != nil {
		return fmt.Errorf("failed to bump dependencies: %w", err)
	}
	res.PrintSummary()

	if res.NothingToUpdate() {
		return nil
	}

	// Show git status to the user.
	gitStatus := exec.CommandContext(ctx, "git", "status")
	gitStatus.Stdout = os.Stdout
	gitStatus.Stderr = os.Stderr
	return gitStatus.Run()
}
