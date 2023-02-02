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

package check

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/onsi/gomega"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/runtime/conditions"
)

// checkFunc is the function type for all the status check functions.
type checkFunc func(ctx context.Context, obj conditions.Getter, condns *Conditions) error

// Checker performs all the status checks. It is configured to provide context
// of the target controller.
type Checker struct {
	// g is used to run the checker as a gomega test helper.
	g *gomega.WithT
	// requireConditions is used to indicate that the checker requires
	// conditions context to operate. It is used to perform validation of the
	// checker instance.
	requireConditions bool
	// K8s client, to fetch the latest version of an object.
	client.Client
	// conditions is the conditions context of the target controller.
	conditions *Conditions
	// failChecks contains all the strict checks.
	failChecks []checkFunc
	// warnChecks contains all the checks that result in warnings.
	warnChecks []checkFunc
	// DisableFetch disables fetching the latest state of an object using the
	// client. This can be used in unit-tests, while passing an object with
	// all the properties to be checked.
	DisableFetch bool
	// Stdout of the checker.
	Stdout io.Writer
	// Stderr of the checker.
	Stderr io.Writer
	// ExcludeChecks contains the checks that should be excluded.
	// TODO: Add support for it in all the checks.
	// ExcludeChecks map[string]bool
}

// NewChecker constructs and returns a new reconciled status Checker for a
// controller.
func NewChecker(cli client.Client, condns *Conditions) *Checker {
	warnChecks := []checkFunc{
		check_WARN0001,
		check_WARN0002,
		check_WARN0003,
		check_WARN0004,
		check_WARN0005,
	}
	failChecks := []checkFunc{
		check_FAIL0001,
		check_FAIL0002,
		check_FAIL0003,
		check_FAIL0004,
		check_FAIL0005,
		check_FAIL0006,
		check_FAIL0007,
		check_FAIL0008,
		check_FAIL0009,
	}
	return &Checker{
		requireConditions: true,
		Client:            cli,
		conditions:        condns,
		warnChecks:        warnChecks,
		failChecks:        failChecks,
		Stdout:            os.Stdout,
		Stderr:            os.Stderr,
	}
}

// NewInProgressChecker constructs and returns a new in-progress status Checker
// for a controller. This exists separatly from NewChecker because the status
// considerations are different for different scenarios, making certain checks
// to not apply when an object is in mid-reconciliation with intermediate
// status values.
func NewInProgressChecker(cli client.Client) *Checker {
	warnChecks := []checkFunc{
		check_WARN0003,
		check_WARN0004,
		check_WARN0005,
	}
	failChecks := []checkFunc{
		check_FAIL0002,
		check_FAIL0004,
		check_FAIL0005,
		check_FAIL0006,
		check_FAIL0011,
	}
	return &Checker{
		Client:     cli,
		warnChecks: warnChecks,
		failChecks: failChecks,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}
}

// WithT takes a *gomega.WithT and returns a Checker that can be used to make
// gomega assertions.
func (c *Checker) WithT(g *gomega.WithT) *Checker {
	c.g = g
	return c
}

// CheckErr performs all the warn and fail checks and prints them to stdout and
// stderr, and exits. This is to be used in CLI.
func (c Checker) CheckErr(ctx context.Context, obj conditions.Getter) {
	if c.g != nil {
		c.g.THelper()
	}
	fail, warn := c.Check(ctx, obj)
	if warn != nil {
		fmt.Fprintf(c.Stdout, "[Check-WARN]: %v\nObserved conditions: %v", warn, obj.GetConditions())
	}
	if fail != nil {
		if c.g == nil {
			fmt.Fprintf(c.Stderr, "[Check-FAIL]: %v\nObserved conditions: %v", fail, obj.GetConditions())
			os.Exit(1)
		}
		c.g.Expect(fail).ToNot(gomega.HaveOccurred(), fmt.Sprintf("[Check-FAIL]: %v\nObserved conditions: %v", fail, obj.GetConditions()))
	}
}

// Check performs all the warn and fail checks and returns the results.
func (c Checker) Check(ctx context.Context, obj conditions.Getter) (fail, warn error) {
	if c.requireConditions && c.conditions == nil {
		return fmt.Errorf("no conditions context provided"), nil
	}
	// Fetch the latest version of the object.
	if !c.DisableFetch {
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return err, nil
		}
	}
	warnErrs := []error{}
	for _, check := range c.warnChecks {
		if err := check(ctx, obj, c.conditions); err != nil {
			warnErrs = append(warnErrs, err)
		}
	}
	warn = kerrors.NewAggregate(warnErrs)
	failErr := []error{}
	for _, check := range c.failChecks {
		if err := check(ctx, obj, c.conditions); err != nil {
			failErr = append(failErr, err)
		}
	}
	fail = kerrors.NewAggregate(failErr)
	return fail, warn
}
