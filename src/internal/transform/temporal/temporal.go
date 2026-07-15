// Package temporal implements the temporal-period transform: it collapses
// commits that fall within a sliding window of N days into single logical change
// sets, so downstream coupling analyses treat closely-timed commits as one.
//
// It is an optional pipeline stage, run after grouping and before analysis when
// --temporal-period is supplied. A window's records all take the window's latest
// calendar day as their rev, and each entity appears at most once per window.
// Because windows overlap (step 1), a physical commit is intentionally counted
// in several windows: correct for logical coupling, wrong for count-based
// analyses.
package temporal

import (
	"time"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

// dateLayout is the canonical YYYY-MM-dd form of every Modification.Date.
const dateLayout = "2006-01-02"

// ErrInvalidPeriod marks a non-positive --temporal-period. It is a usage error
// (exit 2) because the period is supplied directly via a flag.
var ErrInvalidPeriod = terr.New(
	"invalid_temporal_period", 2,
	"--temporal-period must be a positive integer",
	"invalid temporal period",
)

// ErrInvalidDate marks a Modification whose Date is not in YYYY-MM-dd form. It
// is an input error (exit 3) because the offending value comes from the log.
var ErrInvalidDate = terr.New(
	"invalid_temporal_date", 3,
	"generate the log with `codelens print-log-command`",
	"invalid commit date in temporal grouping",
)

// Apply collapses mods into sliding N-day change sets. periodDays must be a
// positive integer; the day range between the first and last commit is padded so
// every calendar day is present, then a window of periodDays (step 1) slides
// across it. Each window merges its commits into one change set whose rev is the
// window's latest day, deduplicated by entity keeping the earliest occurrence;
// entirely empty windows are dropped, as is any incomplete trailing window when
// the range is shorter than the period. The input slice is not mutated.
func Apply(mods []model.Modification, periodDays int) ([]model.Modification, error) {
	if periodDays < 1 {
		return nil, ErrInvalidPeriod.WithDetails(map[string]any{"period_days": periodDays})
	}
	if len(mods) == 0 {
		return []model.Modification{}, nil
	}

	byDate := make(map[string][]model.Modification, len(mods))
	var first, last time.Time
	for i, m := range mods {
		d, err := time.ParseInLocation(dateLayout, m.Date, time.UTC)
		if err != nil {
			return nil, ErrInvalidDate.Wrap(err).WithDetails(map[string]any{"date": m.Date})
		}
		byDate[m.Date] = append(byDate[m.Date], m)
		if i == 0 || d.Before(first) {
			first = d
		}
		if i == 0 || d.After(last) {
			last = d
		}
	}

	var days []string
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		days = append(days, d.Format(dateLayout))
	}

	out := []model.Modification{}
	for start := 0; start+periodDays <= len(days); start++ {
		window := days[start : start+periodDays]
		latest := window[len(window)-1]
		out = append(out, mergeWindow(window, byDate, latest)...)
	}
	return out, nil
}

// mergeWindow flattens the commits of the window's days (ascending) into one
// change set: every record's Rev becomes rev, and each entity is kept only on
// its first (earliest) occurrence. An empty window yields no records.
func mergeWindow(window []string, byDate map[string][]model.Modification, rev string) []model.Modification {
	seen := make(map[string]struct{})
	var merged []model.Modification
	for _, day := range window {
		for _, m := range byDate[day] {
			if _, dup := seen[m.Entity]; dup {
				continue
			}
			seen[m.Entity] = struct{}{}
			m.Rev = rev
			merged = append(merged, m)
		}
	}
	return merged
}
