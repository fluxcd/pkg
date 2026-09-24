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

// Package events provides a Recorder and additional helpers to record Kubernetes Events on an external HTTP endpoint.
//
// # Testing
//
// Controllers that embed the Recorder can test the events they emit using the
// helpers provided here, picking the one that matches the assertion:
//
//   - FakeRecorder records emitted events into a channel for field-level unit
//     assertions (Reason, Action, message, annotations) without any HTTP or
//     apiserver dependency. Use NewNopRecorder to discard events entirely.
//   - TestSink is an in-process HTTP server that captures the Flux event/v1
//     payloads the Recorder posts to its webhook address. Point a real Recorder
//     at TestSink.URL() to assert on the payload shape actually sent to
//     notification-controller.
//   - testenv.WaitForEvents polls the apiserver for the core/v1 Events the
//     Recorder writes, asserting they survive the round-trip. This is the only
//     helper that catches an event silently rejected by admission (for example,
//     a note longer than the apiserver limit on the strict events path).
package events
