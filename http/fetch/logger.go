/*
Copyright 2023 The Flux authors

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

package fetch

import (
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
)

// newErrorLogger returns a retryablehttp.LeveledLogger that only logs
// errors to the given logr.Logger.
func newErrorLogger(logr logr.Logger) retryablehttp.LeveledLogger {
	return &errorLogger{log: logr}
}

// errorLogger is a wrapper around logr.Logger that implements the
// retryablehttp.LeveledLogger interface while only logging errors.
type errorLogger struct {
	log logr.Logger
}

func (l *errorLogger) Error(msg string, keysAndValues ...interface{}) {
	l.log.Info(msg, keysAndValues...)
}

func (l *errorLogger) Info(msg string, keysAndValues ...interface{}) {
	// Do nothing.
}

func (l *errorLogger) Debug(msg string, keysAndValues ...interface{}) {
	// Do nothing.
}

func (l *errorLogger) Warn(msg string, keysAndValues ...interface{}) {
	// Do nothing.
}
