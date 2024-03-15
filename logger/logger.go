package logger

import (
	"fmt"

	"github.com/pubg/kube-image-deployer/interfaces"
	"k8s.io/klog/v2"
)

var _ interfaces.ILogger = (*Logger)(nil)

type Logger struct {
	backend iLogBackend
	depth   int
}

func NewLogger() *Logger {
	return &Logger{
		depth:   1,
		backend: nil, // default=disabled
	}
}

func (l *Logger) SetDepth(depth int) *Logger {
	l.depth = depth
	return l
}

func (l *Logger) WithBackend(backend iLogBackend) *Logger {
	l.backend = backend
	return l
}

func (l *Logger) Infof(format string, args ...interface{}) {
	if !klog.V(2).Enabled() {
		return
	}
	msg := fmt.Sprintf(format, args...)
	klog.InfoDepth(l.depth, msg)
	if l.backend != nil {
		l.backend.InfoDepth(l.depth+1, msg)
	}
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.ErrorDepth(l.depth, msg)
	if l.backend != nil {
		l.backend.ErrorDepth(l.depth+1, msg)
	}
}

func (l *Logger) Warningf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.WarningDepth(l.depth, msg)
	if l.backend != nil {
		l.backend.WarningDepth(l.depth+1, msg)
	}
}
