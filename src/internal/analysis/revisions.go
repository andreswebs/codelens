package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
)

// revisionsRow is one output row of the revisions analysis: how many distinct
// revisions changed an entity, a proxy for its change frequency.
type revisionsRow struct {
	Entity string `json:"entity"`
	NRevs  int    `json:"n_revs"`
}

func init() {
	Register(revisionsDescriptor())
}

// revisionsDescriptor is the registered contract for the revisions analysis. It
// is a function (rather than a package var) so tests can inspect the descriptor
// without depending on process-global registration state.
func revisionsDescriptor() Descriptor {
	return Descriptor{
		Name:    "revisions",
		Summary: "Change frequency per entity",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "n_revs", Type: "int", Desc: "number of distinct revisions"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runRevisions,
	}
}

// runRevisions groups the modifications by entity and counts the distinct
// revisions per entity (a single revision touching an entity more than once is
// counted once). Rows are ordered by revision count descending; entity name
// breaks ties ascending so the ordering is fully deterministic, matching the
// original's [n-revs] desc sort while pinning equal-key order for reproducible
// --rows truncation.
func runRevisions(mods []model.Modification, _ Opts) (any, error) {
	groups := calc.GroupBy(mods, func(m model.Modification) string { return m.Entity })

	rows := make([]revisionsRow, 0, len(groups))
	for _, g := range groups {
		revs := make([]string, 0, len(g.Items))
		for _, m := range g.Items {
			revs = append(revs, m.Rev)
		}
		rows = append(rows, revisionsRow{
			Entity: g.Key,
			NRevs:  len(calc.Distinct(revs)),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].NRevs != rows[j].NRevs {
			return rows[i].NRevs > rows[j].NRevs
		}
		return rows[i].Entity < rows[j].Entity
	})

	return rows, nil
}
