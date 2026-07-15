package analysis

import (
	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
)

// summaryRow is one output row of the summary analysis: a named overview
// statistic and its integer value. The Statistic label is data in kebab-case
// (e.g. "number-of-commits"), matching the original's CSV, and is distinct from
// the snake_case JSON keys.
type summaryRow struct {
	Statistic string `json:"statistic"`
	Value     int    `json:"value"`
}

func init() {
	Register(summaryDescriptor())
}

// summaryDescriptor is the registered contract for the summary analysis. It is a
// function (rather than a package var) so tests can inspect the descriptor
// without depending on process-global registration state.
func summaryDescriptor() Descriptor {
	return Descriptor{
		Name:    "summary",
		Summary: "Overview counts for the mined data",
		RowSchema: []Column{
			{Name: "statistic", Type: "string", Desc: "metric name"},
			{Name: "value", Type: "int", Desc: "metric value"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runSummary,
	}
}

// runSummary emits four overview counts in a fixed order: the number of distinct
// revisions (commits), distinct entities, total modification records (entities
// changed), and distinct authors. The row order and kebab-case labels mirror the
// original so CSV output is byte-comparable.
func runSummary(mods []model.Modification, _ Opts) (any, error) {
	revs := make([]string, 0, len(mods))
	entities := make([]string, 0, len(mods))
	authors := make([]string, 0, len(mods))
	for _, m := range mods {
		revs = append(revs, m.Rev)
		entities = append(entities, m.Entity)
		authors = append(authors, m.Author)
	}

	rows := []summaryRow{
		{Statistic: "number-of-commits", Value: len(calc.Distinct(revs))},
		{Statistic: "number-of-entities", Value: len(calc.Distinct(entities))},
		{Statistic: "number-of-entities-changed", Value: len(mods)},
		{Statistic: "number-of-authors", Value: len(calc.Distinct(authors))},
	}

	return rows, nil
}
