package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// authorChurnRow is one output row of the author-churn analysis: the lines an
// author added and deleted across all their revisions, with the number of
// distinct revisions they authored.
type authorChurnRow struct {
	Author  string `json:"author"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Commits int    `json:"commits"`
}

func init() {
	Register(authorChurnDescriptor())
}

// authorChurnDescriptor is the registered contract for the author-churn
// analysis. It is a function (rather than a package var) so tests can inspect
// the descriptor without depending on process-global registration state.
func authorChurnDescriptor() Descriptor {
	return Descriptor{
		Name:    "author-churn",
		Summary: "Lines added/deleted per author",
		RowSchema: []Column{
			{Name: "author", Type: "string", Desc: "commit author"},
			{Name: "added", Type: "int", Desc: "lines added by the author"},
			{Name: "deleted", Type: "int", Desc: "lines deleted by the author"},
			{Name: "commits", Type: "int", Desc: "number of distinct revisions by the author"},
		},
		ErrorCodes: []string{"empty_log", "missing_metrics"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runAuthorChurn,
	}
}

// runAuthorChurn sums the lines added and deleted per author and counts the
// distinct revisions each authored. It requires loc metrics (a message-only log
// is a missing_metrics input error). Rows are ordered by [author, added]
// ascending, matching the original's sort; authors are unique per group so the
// added tiebreaker only pins equal-author order that cannot occur, keeping the
// sort deterministic.
func runAuthorChurn(mods []model.Modification, _ Opts) (any, error) {
	if err := churn.RequireLoc(mods); err != nil {
		return nil, err
	}

	groups := churn.SumByGroup(mods, func(m model.Modification) string { return m.Author })

	rows := calc.Map(groups, func(g churn.GroupChurn) authorChurnRow {
		return authorChurnRow{
			Author:  g.Group,
			Added:   g.Added,
			Deleted: g.Deleted,
			Commits: g.Commits,
		}
	})

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Author != rows[j].Author {
			return rows[i].Author < rows[j].Author
		}
		return rows[i].Added < rows[j].Added
	})

	return rows, nil
}
