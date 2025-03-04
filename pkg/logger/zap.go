package logger

import (
	envtypes "github.com/openshift-online/maestro/cmd/maestro/environments/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// keep the logger in a singleton mode, allowing for
// dynamic changes to the log level at runtime.
var (
	zapLogLevel zap.AtomicLevel
	zapLogger   *zap.SugaredLogger
)

// GetLogger returns the singleton logger instance, initializing it
// with the given environment if necessary.
func GetLogger() *zap.SugaredLogger {
	if zapLogger == nil {
		zapLogLevel = zap.NewAtomicLevel()
		zapConfig := zap.NewDevelopmentConfig()
		zapConfig.DisableStacktrace = true
		zapConfig.Encoding = "console"
		if envtypes.GetEnvironmentStrFromEnv() == "development" {
			zapLogLevel.SetLevel(zapcore.DebugLevel)
		} else {
			zapLogLevel.SetLevel(zapcore.InfoLevel)
		}
		zapConfig.Level = zapLogLevel
		zlog, err := zapConfig.Build()
		if err != nil {
			panic(err)
		}
		zapLogger = zlog.Sugar()
	}
	return zapLogger
}

// SetLogLevel sets the log level for the logger.
func SetLogLevel(level string) {
	zapLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		zapLogger.Errorf("failed to parse log level: %v", err)
		return
	}
	zapLogLevel.SetLevel(zapLevel)
}

// GetLoggerLevel returns the current log level of the logger.
func GetLoggerLevel() string {
	return zapLogLevel.String()
}

// SyncLogger flushes any buffered log entries.
// This should be called before the application exits.
func SyncLogger() {
	if zapLogger != nil {
		_ = zapLogger.Sync()
	}
}
