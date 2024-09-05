package util

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/klog/v2"
)

type eventRecorder struct {
	source string
}

func NewRecorder(sourceComponent string) events.Recorder {
	return &eventRecorder{source: sourceComponent}
}

func (r *eventRecorder) ComponentName() string {
	return r.source
}

func (r *eventRecorder) Shutdown() {}

func (r *eventRecorder) ForComponent(component string) events.Recorder {
	// do nothing
	return r
}

func (r *eventRecorder) WithContext(ctx context.Context) events.Recorder {
	// do nothing
	return r
}

func (r *eventRecorder) WithComponentSuffix(suffix string) events.Recorder {
	// do nothing
	return r
}

func (r *eventRecorder) Event(reason, message string) {
	klog.V(4).Infof("[%s] reason=%s, message=%s", r.source, reason, message)
}

func (r *eventRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *eventRecorder) Warning(reason, message string) {
	klog.V(2).Infof("[%s] reason=%s, message=%s", r.source, reason, message)
}

func (r *eventRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}
