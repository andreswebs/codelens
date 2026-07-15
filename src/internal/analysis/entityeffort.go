package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/effort"
	"github.com/andreswebs/codelens/internal/model"
)

// entityEffortRow is one output row of the entity-effort analysis: an author's
// revision count within an entity (AuthorRevs) alongside the entity's total
// revision count (TotalRevs), so the author's share is readable without
// re-deriving the total.
type entityEffortRow struct {
	Entity     string `json:"entity"`
	Author     string `json:"author"`
	AuthorRevs int    `json:"author_revs"`
	TotalRevs  int    `json:"total_revs"`
}

func init() {
	Register(entityEffortDescriptor())
}

// entityEffortDescriptor is the registered contract for the entity-effort
// analysis. It is a function (rather than a package var) so tests can inspect
// the descriptor without depending on process-global registration state.
func entityEffortDescriptor() Descriptor {
	return Descriptor{
		Name:    "entity-effort",
		Summary: "Each author revision share per entity",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "author", Type: "string", Desc: "contributing author"},
			{Name: "author_revs", Type: "int", Desc: "revisions the author contributed to the entity"},
			{Name: "total_revs", Type: "int", Desc: "total revisions of the entity across all authors"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runEntityEffort,
	}
}

// runEntityEffort flattens each entity's per-author revision shares into one
// row per (entity, author). effort.ByEntity returns entities and authors in
// ascending key order, so the stable sort below orders rows by entity ascending
// and, within an entity, by author revisions descending while preserving the
// ascending-author order for equal revision counts. This matches the original's
// stable sort-by revs desc then sort-by entity.
func runEntityEffort(mods []model.Modification, _ Opts) (any, error) {
	entities := effort.ByEntity(mods)

	rows := calc.FlatMap(entities, func(e effort.EntityEffort) []entityEffortRow {
		return calc.Map(e.Authors, func(a effort.AuthorRevs) entityEffortRow {
			return entityEffortRow{
				Entity:     e.Entity,
				Author:     a.Author,
				AuthorRevs: a.Revs,
				TotalRevs:  a.TotalRevs,
			}
		})
	})

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Entity != rows[j].Entity {
			return rows[i].Entity < rows[j].Entity
		}
		return rows[i].AuthorRevs > rows[j].AuthorRevs
	})

	return rows, nil
}
