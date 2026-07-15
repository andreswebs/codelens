package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
)

// authorsRow is one output row of the authors analysis: how many distinct
// authors touched an entity and how many revisions it accrued.
type authorsRow struct {
	Entity   string `json:"entity"`
	NAuthors int    `json:"n_authors"`
	NRevs    int    `json:"n_revs"`
}

func init() {
	Register(authorsDescriptor())
}

// authorsDescriptor is the registered contract for the authors analysis. It is
// a function (rather than a package var) so tests can inspect the descriptor
// without depending on process-global registration state.
func authorsDescriptor() Descriptor {
	return Descriptor{
		Name:    "authors",
		Summary: "Number of distinct authors per entity",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "n_authors", Type: "int", Desc: "distinct authors that touched it"},
			{Name: "n_revs", Type: "int", Desc: "revisions of the entity"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runAuthors,
	}
}

// runAuthors groups the modifications by entity, counts the distinct authors
// and the revisions (rows) per entity, and orders the rows by author count then
// revision count descending. Entity name breaks ties ascending so the ordering
// is fully deterministic; the original leaves equal-key order to dataset
// insertion, a divergence documented for reproducible --rows truncation.
func runAuthors(mods []model.Modification, _ Opts) (any, error) {
	groups := calc.GroupBy(mods, func(m model.Modification) string { return m.Entity })

	rows := make([]authorsRow, 0, len(groups))
	for _, g := range groups {
		authors := make([]string, 0, len(g.Items))
		for _, m := range g.Items {
			authors = append(authors, m.Author)
		}
		rows = append(rows, authorsRow{
			Entity:   g.Key,
			NAuthors: len(calc.Distinct(authors)),
			NRevs:    len(g.Items),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].NAuthors != rows[j].NAuthors {
			return rows[i].NAuthors > rows[j].NAuthors
		}
		if rows[i].NRevs != rows[j].NRevs {
			return rows[i].NRevs > rows[j].NRevs
		}
		return rows[i].Entity < rows[j].Entity
	})

	return rows, nil
}
