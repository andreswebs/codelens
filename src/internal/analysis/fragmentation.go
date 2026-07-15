package analysis

import (
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/effort"
	"github.com/andreswebs/codelens/internal/model"
)

// fragmentationRow is one output row of the fragmentation analysis: an entity's
// fractal value (how spread its authorship is, 0 for a single author toward 1
// for many equal contributors) alongside its total revision count.
type fragmentationRow struct {
	Entity       string  `json:"entity"`
	FractalValue float64 `json:"fractal_value"`
	TotalRevs    int     `json:"total_revs"`
}

func init() {
	Register(fragmentationDescriptor())
}

// fragmentationDescriptor is the registered contract for the fragmentation
// analysis. It is a function (rather than a package var) so tests can inspect
// the descriptor without depending on process-global registration state.
func fragmentationDescriptor() Descriptor {
	return Descriptor{
		Name:    "fragmentation",
		Summary: "Author fragmentation (fractal value) per entity",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "fractal_value", Type: "float", Desc: "authorship spread, 0 (one author) toward 1 (many equal authors), 2 significant digits"},
			{Name: "total_revs", Type: "int", Desc: "total revisions of the entity across all authors"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runFragmentation,
	}
}

// runFragmentation computes each entity's fractal value, the authorship-spread
// heuristic 1 - Σ (author_revs / total_revs)², rounded to two significant
// digits to match the original's ratio->centi-float-precision. Rows are sorted
// by fractal value descending, then total revs descending; effort.ByEntity
// returns entities in ascending order, so the stable sort keeps entity-ascending
// order for fully tied rows.
func runFragmentation(mods []model.Modification, _ Opts) (any, error) {
	entities := effort.ByEntity(mods)

	rows := make([]fragmentationRow, 0, len(entities))
	for _, e := range entities {
		total := e.Authors[0].TotalRevs
		var sumSquares float64
		for _, a := range e.Authors {
			share := float64(a.Revs) / float64(total)
			sumSquares += share * share
		}
		rows = append(rows, fragmentationRow{
			Entity:       e.Entity,
			FractalValue: calc.CentiFloat(1 - sumSquares),
			TotalRevs:    total,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].FractalValue != rows[j].FractalValue {
			return rows[i].FractalValue > rows[j].FractalValue
		}
		return rows[i].TotalRevs > rows[j].TotalRevs
	})

	return rows, nil
}
