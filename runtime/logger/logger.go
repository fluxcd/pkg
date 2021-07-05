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
	"github.com/spf13/pflag"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	flagLogEncoding = "log-encoding"
	flagLogLevel    = "log-level"
)

var levelStrings = map[string]zapcore.Level{
	// zap doesn't include trace level as a const, but it accepts any
	// int8; logr will convert a log.V(n) to zap's scheme, so e.g.,
	// V(2) will be custom debug level -2 in zap (i.e., `trace`
	// below).
	"trace": zapcore.DebugLevel - 1,
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"error": zapcore.ErrorLevel,
}

// These are for convenience when doing log.V(...) to log at a
// particular level. They correspond to the logr equivalents of the
// zap levels above.
const (
	TraceLevel = 2
	DebugLevel = 1
	InfoLevel  = 0
)

var stackLevelStrings = map[string]zapcore.Level{
	"trace": zapcore.ErrorLevel,
	"debug": zapcore.ErrorLevel,
	"info":  zapcore.PanicLevel,
	"error": zapcore.PanicLevel,
}

// Options contains the configuration options for the runtime logger.
//
// The struct can be used in the main.go file of your controller by binding it to the main flag set, and then utilizing
// the configured options later:
//
//	func main() {
//		var (
//			// other controller specific configuration variables
//			loggerOptions logger.Options
//		)
//
//		// Bind the options to the main flag set, and parse it
//		loggerOptions.BindFlags(flag.CommandLine)
//		flag.Parse()
//
//		// Use the values during the initialisation of the logger
//		ctrl.SetLogger(logger.NewLogger(logOptions))
//  }
type Options struct {
	LogEncoding string
	LogLevel    string
}

// BindFlags will parse the given pflag.FlagSet for logger option flags and set the Options accordingly.
func (o *Options) BindFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.LogEncoding, flagLogEncoding, "json",
		"Log encoding format. Can be 'json' or 'console'.")
	fs.StringVar(&o.LogLevel, flagLogLevel, "info",
		"Log verbosity level. Can be one of 'trace', 'debug', 'info', 'error'.")
}

// NewLogger returns a logger configured with the given Options, and timestamps set to the ISO8601 format.
func NewLogger(opts Options) logr.Logger {
	zapOpts := zap.Options{
		EncoderConfigOptions: []zap.EncoderConfigOption{
			func(config *zapcore.EncoderConfig) {
				config.EncodeTime = zapcore.ISO8601TimeEncoder
			},
		},
	}

	switch opts.LogEncoding {
	case "console":
		zap.ConsoleEncoder(zapOpts.EncoderConfigOptions...)(&zapOpts)
	case "json":
		zap.JSONEncoder(zapOpts.EncoderConfigOptions...)(&zapOpts)
	}

	if l, ok := levelStrings[opts.LogLevel]; ok {
		zapOpts.Level = l
	}

	if l, ok := stackLevelStrings[opts.LogLevel]; ok {
		zapOpts.StacktraceLevel = l
	}

	return zap.New(zap.UseFlagOptions(&zapOpts))
}
