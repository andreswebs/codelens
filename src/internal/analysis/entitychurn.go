package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/churn"
	"github.com/andreswebs/codelens/internal/model"
)

// entityChurnRow is one output row of the entity-churn analysis: the lines
// added and deleted for a given entity, with the number of distinct commits
// that touched it.
type entityChurnRow struct {
	Entity  string `json:"entity"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Commits int    `json:"commits"`
}

func init() {
	Register(entityChurnDescriptor())
}

// entityChurnDescriptor is the registered contract for the entity-churn
// analysis. It is a function (rather than a package var) so tests can inspect
// the descriptor without depending on process-global registration state.
func entityChurnDescriptor() Descriptor {
	return Descriptor{
		Name:    "entity-churn",
		Summary: "Lines added/deleted per entity",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "added", Type: "int", Desc: "lines added to the entity"},
			{Name: "deleted", Type: "int", Desc: "lines deleted from the entity"},
			{Name: "commits", Type: "int", Desc: "number of distinct revisions touching the entity"},
		},
		ErrorCodes: []string{"empty_log", "missing_metrics"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runEntityChurn,
	}
}

// runEntityChurn sums the lines added and deleted per entity and counts the
// distinct revisions that touched each. It requires loc metrics (a message-only
// log is a missing_metrics input error). Rows are ordered by added lines
// descending, matching the original's sort; entities with equal added lines
// keep ascending entity order (a stable sort over the ascending-key grouping),
// making the output deterministic.
func runEntityChurn(mods []model.Modification, _ Opts) (any, error) {
	if err := churn.RequireLoc(mods); err != nil {
		return nil, err
	}

	groups := churn.SumByGroup(mods, func(m model.Modification) string { return m.Entity })

	rows := calc.Map(groups, func(g churn.GroupChurn) entityChurnRow {
		return entityChurnRow{
			Entity:  g.Group,
			Added:   g.Added,
			Deleted: g.Deleted,
			Commits: g.Commits,
		}
	})

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Added > rows[j].Added
	})

	return rows, nil
}
