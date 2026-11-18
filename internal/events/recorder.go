// Package events publishes replica decisions as Kubernetes events.
package events

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	platformv1 "github.com/bjo3g15nw7/cron-scale-operator/api/v1alpha1"
)

// Reasons published on the CronScale object.
const (
	ReasonWindowApplied = "WindowApplied"
	ReasonNoOpenWindow  = "NoOpenWindow"
	ReasonInvalid       = "EvaluationFailed"
	// ReasonSuspended is used when spec.suspend freezes reconciliation.
	ReasonSuspended = "Suspended"
)

// Recorder wraps an event sink with decision-shaped helpers.
type Recorder struct {
	sink record.EventRecorder
}

// New builds a Recorder around sink.
func New(sink record.EventRecorder) *Recorder {
	return &Recorder{sink: sink}
}

// Decision reports the replica count applied for the current evaluation.
func (r *Recorder) Decision(cs *platformv1.CronScale, window string, replicas int32) {
	if r == nil || r.sink == nil {
		return
	}
	if window == "" {
		r.sink.Event(cs, corev1.EventTypeNormal, ReasonNoOpenWindow,
			fmt.Sprintf("no window open, holding %d replicas", replicas))
		return
	}
	r.sink.Event(cs, corev1.EventTypeNormal, ReasonWindowApplied,
		fmt.Sprintf("window %q holds %d replicas", window, replicas))
}

// Suspended reports that the object was skipped because spec.suspend is set.
func (r *Recorder) Suspended(cs *platformv1.CronScale) {
	if r == nil || r.sink == nil {
		return
	}
	r.sink.Event(cs, corev1.EventTypeNormal, ReasonSuspended,
		"reconciliation frozen by spec.suspend")
}

// Invalid reports a window or target problem as a warning.
func (r *Recorder) Invalid(cs *platformv1.CronScale, cause error) {
	if r == nil || r.sink == nil || cause == nil {
		return
	}
	r.sink.Event(cs, corev1.EventTypeWarning, ReasonInvalid, cause.Error())
}
