package logger

import (
	"context"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/golang/glog"
	"github.com/openshift-online/maestro/pkg/util"
)

type OCMLogger interface {
	V(level int32) OCMLogger
	Infof(format string, args ...interface{})
	Extra(key string, value interface{}) OCMLogger
	Info(message string)
	Warning(message string)
	Error(message string)
	Fatal(message string)
}

var _ OCMLogger = &logger{}

type extra map[string]interface{}

type logger struct {
	context   context.Context
	level     int32
	accountID string
	// TODO username is unused, should we be logging it? Could be pii
	username  string
	sentryHub *sentry.Hub
	extra     extra
}

// NewOCMLogger creates a new logger instance with a default verbosity of 1
func NewOCMLogger(ctx context.Context) OCMLogger {
	logger := &logger{
		context:   ctx,
		level:     1,
		extra:     make(extra),
		accountID: util.GetAccountIDFromContext(ctx),
		sentryHub: sentry.GetHubFromContext(ctx),
	}
	return logger
}

func (l *logger) prepareLogPrefix(message string, extra extra) string {
	prefix := " "

	if txid, ok := l.context.Value("txid").(int64); ok {
		prefix = fmt.Sprintf("[tx_id=%d]%s", txid, prefix)
	}

	if l.accountID != "" {
		prefix = fmt.Sprintf("[accountID=%s]%s", l.accountID, prefix)
	}

	if opid, ok := l.context.Value(OpIDKey).(string); ok {
		prefix = fmt.Sprintf("[opid=%s]%s", opid, prefix)
	}

	args := []string{}
	for k, v := range extra {
		args = append(args, fmt.Sprintf("%s=%v", k, v))
	}

	return fmt.Sprintf("%s %s %s", prefix, message, strings.Join(args, " "))
}

func (l *logger) prepareLogPrefixf(format string, args ...interface{}) string {
	orig := fmt.Sprintf(format, args...)
	prefix := " "

	if txid, ok := l.context.Value("txid").(int64); ok {
		prefix = fmt.Sprintf("[tx_id=%d]%s", txid, prefix)
	}

	if l.accountID != "" {
		prefix = fmt.Sprintf("[accountID=%s]%s", l.accountID, prefix)
	}

	if opid, ok := l.context.Value(OpIDKey).(string); ok {
		prefix = fmt.Sprintf("[opid=%s]%s", opid, prefix)
	}

	return fmt.Sprintf("%s%s", prefix, orig)
}

func (l *logger) V(level int32) OCMLogger {
	return &logger{
		context:   l.context,
		accountID: l.accountID,
		username:  l.username,
		level:     level,
	}
}

// Infof doesn't trigger Sentry error
func (l *logger) Infof(format string, args ...interface{}) {
	prefixed := l.prepareLogPrefixf(format, args...)
	glog.V(glog.Level(l.level)).Infof("%s", prefixed)
}

func (l *logger) Extra(key string, value interface{}) OCMLogger {
	l.extra[key] = value
	return l
}

func (l *logger) Info(message string) {
	l.log(message, sentry.LevelInfo, glog.V(glog.Level(l.level)).Infoln)
}

func (l *logger) Warning(message string) {
	l.log(message, sentry.LevelWarning, glog.Warningln)
}

func (l *logger) Error(message string) {
	l.log(message, sentry.LevelError, glog.Errorln)
}

func (l *logger) Fatal(message string) {
	l.log(message, sentry.LevelFatal, glog.Fatalln)
}

func (l *logger) log(message string, level sentry.Level, glogFunc func(args ...interface{})) {
	prefixed := l.prepareLogPrefix(message, l.extra)
	glogFunc(prefixed)
	if level != sentry.LevelInfo && level != sentry.LevelWarning {
		l.captureSentryEvent(level, message)
	}
}

func (l *logger) captureSentryEvent(level sentry.Level, message string) {
	event := sentry.NewEvent()
	event.Level = level
	event.Message = message
	event.Extra = l.extra
	captureFunc := sentry.CaptureEvent
	if l.sentryHub == nil {
		sentry.CaptureException(fmt.Errorf("sentry hub not present in logger"))
	} else {
		captureFunc = l.sentryHub.CaptureEvent
	}
	captureFunc(event)
}
