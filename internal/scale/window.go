// Package scale decides which scaling window is open at a given instant.
package scale

import (
	"errors"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	platformv1 "github.com/bjo3g15nw7/cron-scale-operator/api/v1alpha1"
)

// lookback bounds how far back the evaluator searches for the previous
// occurrence of a cron expression. Week-granular schedules need six days.
const lookback = 8 * 24 * time.Hour

// ErrNoBoundaries reports a window whose start and end never fire inside the
// lookback horizon.
var ErrNoBoundaries = errors.New("window has no occurrence in lookback horizon")

// Compiled is a parsed, timezone-bound window ready for evaluation.
type Compiled struct {
	Name     string
	start    cron.Schedule
	end      cron.Schedule
	Replicas int32
}

// Compile parses the spec windows in the given zone.
func Compile(windows []platformv1.Window, loc *time.Location) ([]Compiled, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	out := make([]Compiled, 0, len(windows))

	for _, w := range windows {
		start, err := parser.Parse(w.Start)
		if err != nil {
			return nil, fmt.Errorf("window %q start: %w", w.Name, err)
		}
		end, err := parser.Parse(w.End)
		if err != nil {
			return nil, fmt.Errorf("window %q end: %w", w.Name, err)
		}
		if w.Replicas < 0 {
			return nil, fmt.Errorf("window %q: negative replicas %d", w.Name, w.Replicas)
		}
		out = append(out, Compiled{Name: w.Name, start: start, end: end, Replicas: w.Replicas})
	}
	return out, nil
}

// Active reports whether the window is open at now, plus the instant that
// opened it. A window is open when its last start happened after its last end.
func (c *Compiled) Active(now time.Time) (bool, time.Time, error) {
	start, err := previous(c.start, now)
	if errors.Is(err, ErrNoBoundaries) {
		return false, time.Time{}, nil
	}
	if err != nil {
		return false, time.Time{}, err
	}

	end, err := previous(c.end, now)
	if err != nil && !errors.Is(err, ErrNoBoundaries) {
		return false, time.Time{}, err
	}
	if err == nil && !end.Before(start) {
		return false, time.Time{}, nil
	}
	return true, start, nil
}

// Select returns the open window that started most recently, if any.
func Select(windows []Compiled, now time.Time) (*Compiled, error) {
	var (
		winner *Compiled
		newest time.Time
	)
	for i := range windows {
		open, since, err := windows[i].Active(now)
		if err != nil {
			return nil, err
		}
		if open && (winner == nil || since.After(newest)) {
			winner, newest = &windows[i], since
		}
	}
	return winner, nil
}

// Boundary returns the earliest upcoming window transition, or the zero time
// when no window ever fires again.
func Boundary(windows []Compiled, now time.Time) time.Time {
	var next time.Time
	for i := range windows {
		for _, sched := range []cron.Schedule{windows[i].start, windows[i].end} {
			if n := sched.Next(now); !n.IsZero() && (next.IsZero() || n.Before(next)) {
				next = n
			}
		}
	}
	return next
}

// Clamp bounds replicas by the spec limits.
func Clamp(replicas, min, max int32) int32 {
	if max > 0 && replicas > max {
		replicas = max
	}
	if replicas < min {
		replicas = min
	}
	return replicas
}

// previous finds the last occurrence of sched at or before now.
func previous(sched cron.Schedule, now time.Time) (time.Time, error) {
	cursor := now.Add(-lookback)
	var last time.Time
	for range 4000 {
		next := sched.Next(cursor)
		if next.After(now) {
			break
		}
		last = next
		cursor = next
	}
	if last.IsZero() {
		return time.Time{}, ErrNoBoundaries
	}
	return last, nil
}
