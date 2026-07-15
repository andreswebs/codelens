package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// refMainDevRow is one output row of the refactoring-main-developer analysis:
// the author who removed the most lines from an entity, that author's removed
// lines, the entity's total removed lines, and the resulting ownership ratio.
type refMainDevRow struct {
	Entity       string  `json:"entity"`
	MainDev      string  `json:"main_dev"`
	Removed      int     `json:"removed"`
	TotalRemoved int     `json:"total_removed"`
	Ownership    float64 `json:"ownership"`
}

func init() {
	Register(refMainDevDescriptor())
}

// refMainDevDescriptor is the registered contract for the
// refactoring-main-developer analysis. It is a function (rather than a package
// var) so tests can inspect the descriptor without depending on process-global
// registration state.
func refMainDevDescriptor() Descriptor {
	return Descriptor{
		Name:    "refactoring-main-developer",
		Aliases: []string{"refactoring-main-dev"},
		Summary: "Main developer per entity by lines removed",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "main_dev", Type: "string", Desc: "author who removed the most lines"},
			{Name: "removed", Type: "int", Desc: "lines removed by the main developer"},
			{Name: "total_removed", Type: "int", Desc: "total lines removed from the entity by all authors"},
			{Name: "ownership", Type: "float", Desc: "main developer's removed lines over the total (2 significant digits)"},
		},
		ErrorCodes: []string{"empty_log", "missing_metrics"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runRefMainDev,
	}
}

// runRefMainDev picks, per entity, the author who removed the most lines and
// reports their ownership as their removed lines over the entity total. It is
// the deletion-ranked counterpart to main-developer and requires loc metrics (a
// message-only log is a missing_metrics input error). Contributions arrive in
// ascending author order, so the max-remover search keeps the first author on a
// tie, breaking ties by ascending author name. Rows are ordered by entity
// ascending, matching the original's sort.
func runRefMainDev(mods []model.Modification, _ Opts) (any, error) {
	if err := churn.RequireLoc(mods); err != nil {
		return nil, err
	}

	entities := churn.ByEntityAuthorContrib(mods)

	rows := calc.Map(entities, func(e churn.EntityContribs) refMainDevRow {
		top, total := calc.MaxBy(e.Contribs, func(c churn.AuthorContrib) int { return c.Deleted })
		return refMainDevRow{
			Entity:       e.Entity,
			MainDev:      top.Author,
			Removed:      top.Deleted,
			TotalRemoved: total,
			Ownership:    calc.CentiRatio(top.Deleted, total),
		}
	})

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Entity < rows[j].Entity
	})

	return rows, nil
}
