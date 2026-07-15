package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/effort"
	"github.com/andreswebs/codelens/internal/model"
)

// mainDevByRevsRow is one output row of the main-developer-by-revisions
// analysis: the author with the most revisions of an entity, that author's
// revision count, the entity's total revision count, and the resulting
// ownership ratio. The Added/TotalAdded field names mirror the original's
// column names (which count revisions here, not lines) for parity.
type mainDevByRevsRow struct {
	Entity     string  `json:"entity"`
	MainDev    string  `json:"main_dev"`
	Added      int     `json:"added"`
	TotalAdded int     `json:"total_added"`
	Ownership  float64 `json:"ownership"`
}

func init() {
	Register(mainDevByRevsDescriptor())
}

// mainDevByRevsDescriptor is the registered contract for the
// main-developer-by-revisions analysis. It is a function (rather than a package
// var) so tests can inspect the descriptor without depending on process-global
// registration state.
func mainDevByRevsDescriptor() Descriptor {
	return Descriptor{
		Name:    "main-developer-by-revisions",
		Aliases: []string{"main-dev-by-revs"},
		Summary: "Main developer per entity by revision count",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "main_dev", Type: "string", Desc: "author with the most revisions of the entity"},
			{Name: "added", Type: "int", Desc: "revisions by the main developer"},
			{Name: "total_added", Type: "int", Desc: "total revisions of the entity across all authors"},
			{Name: "ownership", Type: "float", Desc: "main developer's revisions over the total (2 significant digits)"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runMainDevByRevs,
	}
}

// runMainDevByRevs picks, per entity, the author with the most revisions and
// reports their ownership as their revisions over the entity total. Unlike
// main-developer, it needs no loc metrics: a revision is one modification row.
// effort.ByEntity returns authors in ascending order, so the max-reviser search
// keeps the first author on a tie, breaking ties by ascending author name. Rows
// are ordered by entity ascending, matching the original's sort.
func runMainDevByRevs(mods []model.Modification, _ Opts) (any, error) {
	entities := effort.ByEntity(mods)

	rows := calc.Map(entities, func(e effort.EntityEffort) mainDevByRevsRow {
		// Use top.TotalRevs (precomputed per author) for the total, not MaxBy's
		// sum, which would double-count the entity-wide total repeated on every
		// author row.
		top, _ := calc.MaxBy(e.Authors, func(a effort.AuthorRevs) int { return a.Revs })
		return mainDevByRevsRow{
			Entity:     e.Entity,
			MainDev:    top.Author,
			Added:      top.Revs,
			TotalAdded: top.TotalRevs,
			Ownership:  calc.CentiRatio(top.Revs, top.TotalRevs),
		}
	})

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Entity < rows[j].Entity
	})

	return rows, nil
}
