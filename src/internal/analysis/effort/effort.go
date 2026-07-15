// Package effort holds the shared aggregation helper for the effort family of
// analyses (entity-effort, main-developer-by-revisions, fragmentation, and
// communication). Every one of those needs each author's revision share of an
// entity, so the package centralizes that roll-up and pins the revision-counting
// rule (a revision is one modification row, matching the original's nrows, not a
// distinct commit) in one place.
package effort

import (
	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
)

// AuthorRevs is one author's revision share of an entity: Revs is the number of
// rows that author contributed to the entity, and TotalRevs is the entity's
// total row count, repeated on every author row so downstream analyses can read
// the share without re-deriving the total.
type AuthorRevs struct {
	Author    string
	Revs      int
	TotalRevs int
}

// EntityEffort is an entity paired with the per-author revision shares of it.
type EntityEffort struct {
	Entity  string
	Authors []AuthorRevs
}

// ByEntity groups mods by entity and, within each entity, by author, counting
// rows at both levels: TotalRevs is the entity's row count and each author's
// Revs is their row count within the entity. This reproduces the original's
// row-based counting (nrows for the total, frequencies for the per-author
// share), so a file listed twice in one change set counts twice. Both the
// entity and author levels come back in ascending key order for deterministic
// downstream sorting and truncation.
func ByEntity(mods []model.Modification) []EntityEffort {
	entities := calc.GroupBy(mods, func(m model.Modification) string { return m.Entity })
	out := make([]EntityEffort, 0, len(entities))
	for _, e := range entities {
		total := len(e.Items)
		authors := calc.GroupBy(e.Items, func(m model.Modification) string { return m.Author })
		revs := make([]AuthorRevs, 0, len(authors))
		for _, a := range authors {
			revs = append(revs, AuthorRevs{Author: a.Key, Revs: len(a.Items), TotalRevs: total})
		}
		out = append(out, EntityEffort{Entity: e.Key, Authors: revs})
	}
	return out
}
