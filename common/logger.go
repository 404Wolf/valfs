package common

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Set up a logger for valfs. If logFile is an emtpy string then do not log to
// a file, if stdout is true then log to standard out as well.
func (c *Client) setupLogger(logFile string, stdout bool) *zap.SugaredLogger {
	// Create encoders
	consoleEncoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		EncodeLevel:   zapcore.CapitalColorLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
	})

	cores := []zapcore.Core{}

	// Start with console core
	if stdout {
		cores = append(cores,
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel),
		)
	}

	// Add file logging if LogFile is specified
	if logFile != "" {
		logFile, _ := os.Create(logFile)

		fileEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:       "time",
			LevelKey:      "level",
			NameKey:       "logger",
			CallerKey:     "caller",
			MessageKey:    "msg",
			StacktraceKey: "stacktrace",
			EncodeLevel:   zapcore.CapitalLevelEncoder,
			EncodeTime:    zapcore.ISO8601TimeEncoder,
			EncodeCaller:  zapcore.ShortCallerEncoder,
		})

		cores = append(cores, zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(logFile),
			zapcore.DebugLevel,
		))
	}

	// Create multi-core logger
	core := zapcore.NewTee(cores...)
	logger := zap.New(core)

	return logger.Sugar()
}
