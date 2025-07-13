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

package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

// ModuleBump represents a module version bump operation and
// helps applying the bump to target modules in the repository.
type ModuleBump struct {
	module     string
	oldVersion string
	newVersion string
	regex      *regexp.Regexp
}

// NewModuleBump creates a ModuleBump for the given module with the old and new versions.
func NewModuleBump(module, oldVersion, newVersion string) (*ModuleBump, error) {
	pattern := fmt.Sprintf(`github\.com/fluxcd/pkg/%s v([^\s]+)`, module)
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex for module %s: %w", module, err)
	}
	return &ModuleBump{
		module:     module,
		oldVersion: oldVersion,
		newVersion: newVersion,
		regex:      regex,
	}, nil
}

// NewModuleBumpForNewModule creates a ModuleBump for a new module with an initial version.
func NewModuleBumpForNewModule(module string) (*ModuleBump, error) {
	return NewModuleBump(module, "", "0.1.0")
}

// String implements the fmt.Stringer interface for *ModuleBump.
func (m *ModuleBump) String() string {
	from := "new module"
	if m.oldVersion != "" {
		from = m.oldVersion
	}
	return fmt.Sprintf("%s: %s => %s", m.module, from, m.newVersion)
}

// Apply replaces the module version in the given target module.
func (m *ModuleBump) Apply(ctx context.Context, targetModule string) (bool, error) {
	const dryRun = false
	return m.apply(ctx, targetModule, dryRun)
}

// DryRunApply replaces the module version in the given target module without writing changes.
func (m *ModuleBump) DryRunApply(ctx context.Context, targetModule string) (bool, error) {
	const dryRun = true
	return m.apply(ctx, targetModule, dryRun)
}

// apply replaces the module version in the given target module.
func (m *ModuleBump) apply(ctx context.Context, targetModule string, dryRun bool) (bool, error) {
	gomod := fmt.Sprintf("%s/go.mod", targetModule)
	b, err := os.ReadFile(gomod)
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", gomod, err)
	}
	oldContent := string(b)
	if !m.regex.MatchString(oldContent) {
		return false, nil
	}
	bumpString := fmt.Sprintf("github.com/fluxcd/pkg/%s v%s", m.module, m.newVersion)
	newContent := m.regex.ReplaceAllString(oldContent, bumpString)
	if oldContent == newContent {
		return false, nil
	}
	if !dryRun {
		if err := os.WriteFile(gomod, []byte(newContent), 0644); err != nil {
			return false, fmt.Errorf("failed to write %s: %w", gomod, err)
		}
		gomodtidy := exec.CommandContext(ctx, "go", "mod", "tidy")
		gomodtidy.Dir = targetModule
		b, err = gomodtidy.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("failed to run go mod tidy in %s: %w\n%s", targetModule, err, string(b))
		}
	}
	return true, nil
}
