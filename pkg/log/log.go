package log

import (
	"context"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type log interface {
	Error(err error, msg string, keysAndValues ...interface{})
	Warning(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

type Logger struct {
	logr.Logger
}

func (l *Logger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.Logger.Error(err, msg, keysAndValues...)
}

func (l *Logger) Warning(msg string, keysAndValues ...interface{}) {
	l.Logger.V(2).Info(msg, keysAndValues...)
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.Logger.Info(msg, keysAndValues...)
}

func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Logger.V(1).Info(msg, keysAndValues...)
}

func GetLogger(ctx context.Context) Logger {
	return Logger{
		ctrl.LoggerFrom(ctx),
	}
}
