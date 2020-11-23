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

package logger

import (
	"github.com/go-logr/logr"
	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// NewLogger returns a logger configured for dev or production use.
// For production the log format is JSON, the timestamps format is ISO8601
// and stack traces are logged when the level is set to debug.
func NewLogger(level string, production bool) logr.Logger {
	if !production {
		return zap.New(zap.UseDevMode(true))
	}

	encCfg := uzap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zap.Encoder(zapcore.NewJSONEncoder(encCfg))

	logLevel := zap.Level(zapcore.InfoLevel)
	stacktraceLevel := zap.StacktraceLevel(zapcore.PanicLevel)

	switch level {
	case "debug":
		logLevel = zap.Level(zapcore.DebugLevel)
		stacktraceLevel = zap.StacktraceLevel(zapcore.ErrorLevel)
	case "error":
		logLevel = zap.Level(zapcore.ErrorLevel)
	}

	return zap.New(encoder, logLevel, stacktraceLevel)
}
