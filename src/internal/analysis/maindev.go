package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// mainDevRow is one output row of the main-developer analysis: the author who
// added the most lines to an entity, that author's added lines, the entity's
// total added lines, and the resulting ownership ratio.
type mainDevRow struct {
	Entity     string  `json:"entity"`
	MainDev    string  `json:"main_dev"`
	Added      int     `json:"added"`
	TotalAdded int     `json:"total_added"`
	Ownership  float64 `json:"ownership"`
}

func init() {
	Register(mainDevDescriptor())
}

// mainDevDescriptor is the registered contract for the main-developer analysis.
// It is a function (rather than a package var) so tests can inspect the
// descriptor without depending on process-global registration state.
func mainDevDescriptor() Descriptor {
	return Descriptor{
		Name:    "main-developer",
		Aliases: []string{"main-dev"},
		Summary: "Main developer per entity by lines added",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "main_dev", Type: "string", Desc: "author who added the most lines"},
			{Name: "added", Type: "int", Desc: "lines added by the main developer"},
			{Name: "total_added", Type: "int", Desc: "total lines added to the entity by all authors"},
			{Name: "ownership", Type: "float", Desc: "main developer's added lines over the total (2 significant digits)"},
		},
		ErrorCodes: []string{"empty_log", "missing_metrics"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runMainDev,
	}
}

// runMainDev picks, per entity, the author who added the most lines and reports
// their ownership as their added lines over the entity total. It requires loc
// metrics (a message-only log is a missing_metrics input error). Contributions
// arrive in ascending author order, so the max-adder search keeps the first
// author on a tie, breaking ties by ascending author name. Rows are ordered by
// entity ascending, matching the original's sort.
func runMainDev(mods []model.Modification, _ Opts) (any, error) {
	if err := churn.RequireLoc(mods); err != nil {
		return nil, err
	}

	entities := churn.ByEntityAuthorContrib(mods)

	rows := calc.Map(entities, func(e churn.EntityContribs) mainDevRow {
		top, total := calc.MaxBy(e.Contribs, func(c churn.AuthorContrib) int { return c.Added })
		return mainDevRow{
			Entity:     e.Entity,
			MainDev:    top.Author,
			Added:      top.Added,
			TotalAdded: total,
			Ownership:  calc.CentiRatio(top.Added, total),
		}
	})

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Entity < rows[j].Entity
	})

	return rows, nil
}
