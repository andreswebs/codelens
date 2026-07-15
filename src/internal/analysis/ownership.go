package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// ownershipRow is one output row of the entity-ownership analysis: the lines a
// single author added to and deleted from a given entity, summed across that
// author's revisions of the entity.
type ownershipRow struct {
	Entity  string `json:"entity"`
	Author  string `json:"author"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
}

func init() {
	Register(ownershipDescriptor())
}

// ownershipDescriptor is the registered contract for the entity-ownership
// analysis. It is a function (rather than a package var) so tests can inspect
// the descriptor without depending on process-global registration state.
func ownershipDescriptor() Descriptor {
	return Descriptor{
		Name:    "entity-ownership",
		Summary: "Per-author churn contribution to each entity",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "author", Type: "string", Desc: "author who contributed to the entity"},
			{Name: "added", Type: "int", Desc: "lines the author added to the entity"},
			{Name: "deleted", Type: "int", Desc: "lines the author deleted from the entity"},
		},
		ErrorCodes: []string{"empty_log", "missing_metrics"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runOwnership,
	}
}

// runOwnership emits one row per (entity, author) with the lines that author
// added and deleted, summed across their revisions of the entity. It requires
// loc metrics (a message-only log is a missing_metrics input error).
// Contributions arrive grouped by entity ascending and, within each entity, by
// author ascending; a stable sort by entity preserves that author order,
// matching the original's entity-ascending sort while keeping ties
// deterministic.
func runOwnership(mods []model.Modification, _ Opts) (any, error) {
	if err := churn.RequireLoc(mods); err != nil {
		return nil, err
	}

	entities := churn.ByEntityAuthorContrib(mods)

	rows := calc.FlatMap(entities, func(e churn.EntityContribs) []ownershipRow {
		return calc.Map(e.Contribs, func(c churn.AuthorContrib) ownershipRow {
			return ownershipRow{
				Entity:  e.Entity,
				Author:  c.Author,
				Added:   c.Added,
				Deleted: c.Deleted,
			}
		})
	})

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Entity < rows[j].Entity
	})

	return rows, nil
}
