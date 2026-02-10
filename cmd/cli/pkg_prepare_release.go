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
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/fluxcd/pkg/cmd/internal"
)

var pkgPrepareReleaseCmd = &cobra.Command{
	Use:   "prep",
	Short: "Prepare release for the github.com/fluxcd/pkg repository Go modules",
	RunE:  runPrepareRelease,
}

var pkgPrepareReleaseCmdFlags struct {
	yes bool
}

func init() {
	pkgCmd.AddCommand(pkgPrepareReleaseCmd)

	pkgPrepareReleaseCmd.Flags().BoolVar(&pkgPrepareReleaseCmdFlags.yes, "yes", false,
		"Skip confirmation prompt and apply changes directly. Use with caution.")
	pkgPrepareReleaseCmd.Flags().MarkHidden("yes")
}

func runPrepareRelease(cmd *cobra.Command, args []string) error {
	ctx := setupSignalHandler()

	res, err := internal.ComputeModuleBumps(ctx)
	if err != nil {
		return fmt.Errorf("failed to compute module bumps: %w", err)
	}
	if !res.MustBumpInternalModules() {
		fmt.Println("No bumps needed.")
		return nil
	}
	res.PrintBumps()

	// Prompt for confirmation to apply changes.
	if !pkgPrepareReleaseCmdFlags.yes {
		fmt.Println("\nConfirm applying changes above to file system? (Y/n, only uppercase Y will confirm)")
		var response string
		fmt.Scanln(&response)
		if response != "Y" {
			fmt.Println("Aborting changes.")
			return nil
		}
	}

	// Apply changes to the file system.
	if err := res.ApplyInternalBumps(ctx); err != nil {
		return fmt.Errorf("failed to apply module bumps: %w", err)
	}

	// Show git status to the user.
	gitStatus := exec.CommandContext(ctx, "git", "status")
	gitStatus.Stdout = os.Stdout
	gitStatus.Stderr = os.Stderr
	return gitStatus.Run()
}
