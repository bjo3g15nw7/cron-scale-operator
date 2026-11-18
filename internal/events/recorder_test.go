// Package events_test covers the decision event helpers.
package events_test

import (
	"errors"
	"strings"
	"testing"

	"k8s.io/client-go/tools/record"

	platformv1 "github.com/bjo3g15nw7/cron-scale-operator/api/v1alpha1"
	"github.com/bjo3g15nw7/cron-scale-operator/internal/events"
)

func newRecorder() (*events.Recorder, *record.FakeRecorder) {
	fake := record.NewFakeRecorder(4)
	return events.New(fake), fake
}

func TestDecisionReportsWindow(t *testing.T) {
	rec, fake := newRecorder()
	rec.Decision(&platformv1.CronScale{}, "business", 24)

	msg := <-fake.Events
	for _, want := range []string{"Normal", "WindowApplied", "business", "24"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("event %q missing %q", msg, want)
		}
	}
}

func TestDecisionWithoutWindowIsIdle(t *testing.T) {
	rec, fake := newRecorder()
	rec.Decision(&platformv1.CronScale{}, "", 2)

	if msg := <-fake.Events; !strings.Contains(msg, "NoOpenWindow") {
		t.Fatalf("event %q should report idle", msg)
	}
}

func TestInvalidIsWarning(t *testing.T) {
	rec, fake := newRecorder()
	rec.Invalid(&platformv1.CronScale{}, errors.New("bad cron"))

	if msg := <-fake.Events; !strings.Contains(msg, "Warning") || !strings.Contains(msg, "bad cron") {
		t.Fatalf("event %q should be a warning with the cause", msg)
	}
}

func TestNilRecorderIsSafe(t *testing.T) {
	var rec *events.Recorder
	rec.Decision(&platformv1.CronScale{}, "business", 1)
	rec.Invalid(&platformv1.CronScale{}, nil)
}
