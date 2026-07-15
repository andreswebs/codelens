package analysis

import (
	"sort"
	"time"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

// dateLayout is the canonical YYYY-MM-dd form the gitlog parser emits and the
// form --time-now must use. Dates are interpreted in UTC so age is reproducible
// across machines and time zones.
const dateLayout = "2006-01-02"

// ErrInvalidTimeNow marks a --time-now value that is not a YYYY-MM-dd date. It
// is a usage error (exit code 2): the caller must supply a valid calendar date
// or omit the flag to use today.
var ErrInvalidTimeNow = terr.New(
	"invalid_time_now",
	2,
	"provide --time-now as a YYYY-MM-dd date, or omit it to use today",
	"invalid --time-now value",
)

// codeAgeRow is one output row of the code-age analysis: an entity and how many
// whole calendar months have elapsed since its most recent change before now.
type codeAgeRow struct {
	Entity    string `json:"entity"`
	AgeMonths int    `json:"age_months"`
}

func init() {
	Register(codeAgeDescriptor())
}

// codeAgeDescriptor is the registered contract for the code-age analysis. It is
// a function (rather than a package var) so tests can inspect the descriptor
// without depending on process-global registration state.
func codeAgeDescriptor() Descriptor {
	return Descriptor{
		Name:    "code-age",
		Aliases: []string{"age"},
		Summary: "Age in months since last modification",
		Flags: []Flag{
			{Name: "time-now", Type: "string", Default: "", Required: false, Desc: "YYYY-MM-dd \"time zero\" for age (default: today, UTC)"},
		},
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "age_months", Type: "int", Desc: "whole calendar months since the last change before time-now"},
		},
		ErrorCodes: []string{"empty_log", "invalid_time_now"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runCodeAge,
	}
}

// runCodeAge reports, per entity, the whole calendar months since its most
// recent change strictly before now. now is --time-now (UTC) or today when the
// flag is absent; a malformed --time-now is an invalid_time_now usage error. An
// entity whose only changes fall on or after now has no age and is dropped.
// Rows are ordered by age ascending (freshest first), with entity name breaking
// ties ascending so the ordering is fully deterministic for --rows truncation.
func runCodeAge(mods []model.Modification, opts Opts) (any, error) {
	now, err := resolveNow(opts.TimeNow)
	if err != nil {
		return nil, err
	}

	groups := calc.GroupBy(mods, func(m model.Modification) string { return m.Entity })

	rows := make([]codeAgeRow, 0, len(groups))
	for _, g := range groups {
		latest, ok := latestBefore(g.Items, now)
		if !ok {
			continue
		}
		rows = append(rows, codeAgeRow{
			Entity:    g.Key,
			AgeMonths: monthsBetween(latest, now),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].AgeMonths != rows[j].AgeMonths {
			return rows[i].AgeMonths < rows[j].AgeMonths
		}
		return rows[i].Entity < rows[j].Entity
	})

	return rows, nil
}

// resolveNow parses the "time zero" date. An empty value means today's UTC
// calendar date (time-of-day discarded). A non-empty value must be YYYY-MM-dd;
// anything else is an invalid_time_now usage error carrying the offending value.
func resolveNow(timeNow string) (time.Time, error) {
	if timeNow == "" {
		n := time.Now().UTC()
		return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse(dateLayout, timeNow)
	if err != nil {
		return time.Time{}, ErrInvalidTimeNow.Wrap(err).WithDetails(map[string]any{"time_now": timeNow})
	}
	return t, nil
}

// latestBefore returns the most recent modification date strictly before now,
// reporting false when none of the group's changes qualify. Dates that fail to
// parse are skipped; the gitlog parser guarantees the canonical form, so this
// only guards against malformed synthetic input rather than masking real data.
func latestBefore(mods []model.Modification, now time.Time) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, m := range mods {
		d, err := time.Parse(dateLayout, m.Date)
		if err != nil || !d.Before(now) {
			continue
		}
		if !found || d.After(latest) {
			latest = d
			found = true
		}
	}
	return latest, found
}

// monthsBetween returns the number of whole calendar months from the earlier
// date to now, matching clj-time's in-months: the raw year/month field
// difference, less one when now's day-of-month precedes the earlier date's (the
// final month has not fully elapsed).
func monthsBetween(from, now time.Time) int {
	months := (now.Year()-from.Year())*12 + int(now.Month()-from.Month())
	if now.Day() < from.Day() {
		months--
	}
	return months
}
