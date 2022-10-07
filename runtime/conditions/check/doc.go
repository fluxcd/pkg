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

// Package check provides warning and failure checks for the status of K8s
// object based on the various properties of the object status. It's mostly
// based on kstatus and helps verify if the object status adheres to the kstatus
// standards.
//
// These checks are documented in detail in https://gist.github.com/darkowlzz/30c31f2e81c48b20398edc082d4fcc96.
//
// Example usage:
//
//	import (
//	    ...
//	    "github.com/fluxcd/pkg/runtime/conditions/check"
//	)
//
//	func TestFoo() {
//	    obj := &testapi.Obj{}
//	    obj.Name = "test-obj"
//	    obj.Namespace = "test-ns"
//
//	    // Initialize the environment, create the object.
//	    ...
//
//	    // Create a status checker with context about the controller. In this
//	    // case, TestCondition1 and TestCondition2 are the negative polarity
//	    // conditions supported by the Obj controller.
//	    conditions := &check.Conditions{NegativePolarity: []string{"TestCondition1", "TestCondition2"}}
//	    checker := check.NewChecker(client, conditions)
//
//	    // Check object status.
//	    checker.CheckErr(context.TODO(), obj)
//	}
//
// Example result:
//
//	[Check-WARN]: Ready condition should have the value of the negative polarity conditon that's present with the highest priority: Ready != TestCondition1
//	Diff:
//	 {
//	- Reason: "SomeReason",
//	- Message: "SomeMsg",
//	+ Reason: "Rsn",
//	+ Message: "Msg",
//	 }
//
// In the above example result, the Ready condition values don't match with the
// negative poliarity condition values. A diff is shown, comparing the Ready
// condition values and the TestCondition1 condition values that don't match.
//
// Another example result:
//
//	[Check-FAIL]: Ready condition must always be present
//
// The above result shows a failure due to a strict check. The Ready condition
// is expected to always be present.
//
// When used in unit-tests, the checker can be configured to disable fetching
// a new version of the object using the K8s client and analyze the given
// object.
//
//	checker.DisableFetch = true
package check
