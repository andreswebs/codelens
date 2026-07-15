package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
)

// socRow is one output row of the sum-of-coupling analysis: an entity and the
// total number of shared transactions (co-changes) it participated in across the
// whole history.
type socRow struct {
	Entity string `json:"entity"`
	Soc    int    `json:"soc"`
}

func init() {
	Register(socDescriptor())
}

// socDescriptor is the registered contract for the sum-of-coupling analysis. It
// is a function (rather than a package var) so tests can inspect the descriptor
// without depending on process-global registration state.
func socDescriptor() Descriptor {
	return Descriptor{
		Name:    "sum-of-coupling",
		Aliases: []string{"soc"},
		Summary: "Sum of coupling per entity",
		Flags: []Flag{
			{Name: "min-revs", Type: "int", Default: 5, Desc: "minimum sum-of-coupling (exclusive) for an entity to be included"},
		},
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "soc", Type: "int", Desc: "number of shared transactions"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runSoc,
	}
}

// runSoc computes each entity's sum-of-coupling: for every revision's change set
// of distinct entities of size k, each member gains k-1 (the number of other
// entities it co-changed with), summed across all revisions. Unlike coupling,
// there is no max-changeset-size filter. Entities are kept only when their sum
// strictly exceeds min-revs (a strict >, unlike coupling's inclusive >=), and
// the survivors are ordered by soc then entity descending, matching the port
// reference's deterministic sort.
func runSoc(mods []model.Modification, opts Opts) (any, error) {
	groups := calc.GroupBy(mods, func(m model.Modification) string { return m.Rev })

	soc := make(map[string]int)
	for _, g := range groups {
		entities := make([]string, 0, len(g.Items))
		for _, m := range g.Items {
			entities = append(entities, m.Entity)
		}
		set := calc.Distinct(entities)
		gain := len(set) - 1
		for _, e := range set {
			soc[e] += gain
		}
	}

	rows := make([]socRow, 0, len(soc))
	for entity, n := range soc {
		if n > opts.MinRevs {
			rows = append(rows, socRow{Entity: entity, Soc: n})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Soc != rows[j].Soc {
			return rows[i].Soc > rows[j].Soc
		}
		return rows[i].Entity > rows[j].Entity
	})

	return rows, nil
}
