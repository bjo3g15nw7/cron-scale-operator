// Package scale_test covers window evaluation and clamping.
package scale_test

import (
	"errors"
	"testing"
	"time"

	platformv1 "github.com/bjo3g15nw7/cron-scale-operator/api/v1alpha1"
	"github.com/bjo3g15nw7/cron-scale-operator/internal/scale"
)

func mustCompile(t *testing.T, windows ...platformv1.Window) []scale.Compiled {
	t.Helper()
	out, err := scale.Compile(windows, time.UTC)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return out
}

func TestSelectPicksOpenWindow(t *testing.T) {
	// Business hours 08:00-18:00 weekdays, night shift 22:00-06:00.
	windows := mustCompile(t,
		platformv1.Window{Name: "business", Start: "0 8 * * 1-5", End: "0 18 * * 1-5", Replicas: 10},
		platformv1.Window{Name: "night", Start: "0 22 * * *", End: "0 6 * * *", Replicas: 4},
	)

	cases := map[string]struct {
		at   time.Time
		want string
	}{
		"mid business day": {time.Date(2026, 3, 4, 11, 0, 0, 0, time.UTC), "business"},
		"overnight":        {time.Date(2026, 3, 5, 2, 0, 0, 0, time.UTC), "night"},
		"between windows":  {time.Date(2026, 3, 4, 19, 0, 0, 0, time.UTC), ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := scale.Select(windows, tc.at)
			if err != nil {
				t.Fatalf("select: %v", err)
			}
			gotName := ""
			if got != nil {
				gotName = got.Name
			}
			if gotName != tc.want {
				t.Fatalf("at %s: got %q, want %q", tc.at, gotName, tc.want)
			}
		})
	}
}

func TestCompileRejectsBadExpression(t *testing.T) {
	_, err := scale.Compile([]platformv1.Window{{Name: "bad", Start: "not a cron", End: "0 18 * * *"}}, time.UTC)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestBoundaryIsNextTransition(t *testing.T) {
	windows := mustCompile(t, platformv1.Window{Name: "business", Start: "0 8 * * *", End: "0 18 * * *", Replicas: 2})
	now := time.Date(2026, 3, 4, 17, 30, 0, 0, time.UTC)

	if got, want := scale.Boundary(windows, now), now.Add(30*time.Minute); !got.Equal(want) {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestClamp(t *testing.T) {
	if got := scale.Clamp(12, 2, 8); got != 8 {
		t.Fatalf("upper bound: got %d", got)
	}
	if got := scale.Clamp(0, 2, 8); got != 2 {
		t.Fatalf("lower bound: got %d", got)
	}
}

func TestPreviousWithoutOccurrence(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	if _, err := scale.Select(nil, now); err != nil && !errors.Is(err, scale.ErrNoBoundaries) {
		t.Fatalf("select on empty list: %v", err)
	}
}
