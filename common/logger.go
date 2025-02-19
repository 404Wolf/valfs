package common

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

// SetupLogger configures a Zap logger with console and/or file output.
// Parameters:
//   - logFile: path to log file (empty string disables file logging)
//   - logLevel: minimum log level ("debug", "info", "warn", "error")
//   - silent: if true, disables console output
//
// Returns a configured SugaredLogger instance.
func SetupLogger(logFile string, logLevel string, silent bool) *zap.SugaredLogger {
	// Convert string level to zapcore.Level
	level := zapcore.InfoLevel // default to info
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	cores := []zapcore.Core{}

	// Add console logging if not silent
	if !silent {
		consoleConfig := zapcore.EncoderConfig{
			TimeKey:       "time",
			LevelKey:      "level",
			NameKey:       "logger",
			CallerKey:     "caller",
			MessageKey:    "msg",
			StacktraceKey: "stacktrace",
			EncodeLevel:   zapcore.CapitalColorLevelEncoder,
			EncodeTime:    zapcore.ISO8601TimeEncoder,
			EncodeCaller:  zapcore.ShortCallerEncoder,
		}
		consoleEncoder := zapcore.NewConsoleEncoder(consoleConfig)
		cores = append(cores,
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level),
		)
	}

	// Add file logging if logFile is specified
	if logFile != "" {
		fileConfig := zapcore.EncoderConfig{
			TimeKey:       "time",
			LevelKey:      "level",
			NameKey:       "logger",
			CallerKey:     "caller",
			MessageKey:    "msg",
			StacktraceKey: "stacktrace",
			EncodeLevel:   zapcore.CapitalLevelEncoder,
			EncodeTime:    zapcore.ISO8601TimeEncoder,
			EncodeCaller:  zapcore.ShortCallerEncoder,
		}
		logFile, _ := os.Create(logFile)
		fileEncoder := zapcore.NewJSONEncoder(fileConfig)
		cores = append(cores,
			zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), level),
		)
	}

	// Create multi-core logger
	core := zapcore.NewTee(cores...)
	logger := zap.New(core)

	return logger.Sugar()
}
