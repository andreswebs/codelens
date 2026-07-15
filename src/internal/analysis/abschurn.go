package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// absChurnRow is one output row of the absolute-churn analysis: the lines added
// and deleted on a given date, with the number of distinct commits that made
// those changes.
type absChurnRow struct {
	Date    string `json:"date"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Commits int    `json:"commits"`
}

func init() {
	Register(absChurnDescriptor())
}

// absChurnDescriptor is the registered contract for the absolute-churn
// analysis. It is a function (rather than a package var) so tests can inspect
// the descriptor without depending on process-global registration state.
func absChurnDescriptor() Descriptor {
	return Descriptor{
		Name:    "absolute-churn",
		Aliases: []string{"abs-churn"},
		Summary: "Lines added/deleted per date",
		RowSchema: []Column{
			{Name: "date", Type: "string", Desc: "commit date (YYYY-MM-dd)"},
			{Name: "added", Type: "int", Desc: "lines added on the date"},
			{Name: "deleted", Type: "int", Desc: "lines deleted on the date"},
			{Name: "commits", Type: "int", Desc: "number of distinct revisions on the date"},
		},
		ErrorCodes: []string{"empty_log", "missing_metrics"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runAbsChurn,
	}
}

// runAbsChurn sums the lines added and deleted per calendar date and counts the
// distinct revisions on each date. It requires loc metrics (a message-only log
// is a missing_metrics input error). Rows are ordered by [date, added, deleted]
// ascending, matching the original's sort; dates are unique per group so the
// loc tiebreakers only pin equal-date order that cannot occur, keeping the sort
// deterministic.
func runAbsChurn(mods []model.Modification, _ Opts) (any, error) {
	if err := churn.RequireLoc(mods); err != nil {
		return nil, err
	}

	groups := churn.SumByGroup(mods, func(m model.Modification) string { return m.Date })

	rows := calc.Map(groups, func(g churn.GroupChurn) absChurnRow {
		return absChurnRow{
			Date:    g.Group,
			Added:   g.Added,
			Deleted: g.Deleted,
			Commits: g.Commits,
		}
	})

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		if rows[i].Added != rows[j].Added {
			return rows[i].Added < rows[j].Added
		}
		return rows[i].Deleted < rows[j].Deleted
	})

	return rows, nil
}
