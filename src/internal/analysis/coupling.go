package analysis

import (
	"fmt"
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/analysis/couplingalgo"
	"github.com/andreswebs/codelens/internal/model"
)

// couplingRow is one output row of the coupling analysis: a coupled entity pair,
// their coupling degree (percent), and their average revisions. The three
// pointer columns are populated only under --verbose and are omitted otherwise,
// so a standard result carries exactly the four documented columns.
type couplingRow struct {
	Entity      string `json:"entity"`
	Coupled     string `json:"coupled"`
	Degree      int    `json:"degree"`
	AverageRevs int    `json:"average_revs"`

	FirstEntityRevisions  *int `json:"first_entity_revisions,omitempty"`
	SecondEntityRevisions *int `json:"second_entity_revisions,omitempty"`
	SharedRevisions       *int `json:"shared_revisions,omitempty"`
}

func init() {
	Register(couplingDescriptor())
}

// couplingDescriptor is the registered contract for the coupling analysis. It is
// a function (rather than a package var) so tests can inspect the descriptor
// without depending on process-global registration state.
func couplingDescriptor() Descriptor {
	return Descriptor{
		Name:    "coupling",
		Summary: "Logical (temporal) coupling between entity pairs",
		Flags: []Flag{
			{Name: "min-revs", Type: "int", Default: 5, Desc: "minimum revisions for a pair's average to be included"},
			{Name: "min-shared-revs", Type: "int", Default: 5, Desc: "minimum shared revisions for a pair"},
			{Name: "min-coupling", Type: "int", Default: 30, Desc: "minimum coupling degree in percent"},
			{Name: "max-coupling", Type: "int", Default: 100, Desc: "maximum coupling degree in percent"},
			{Name: "max-changeset-size", Type: "int", Default: 30, Desc: "skip change sets larger than this size"},
			{Name: "verbose", Type: "bool", Default: false, Desc: "add per-pair revision detail columns"},
		},
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "coupled", Type: "string", Desc: "co-changing module path"},
			{Name: "degree", Type: "int", Desc: "coupling strength, percent 0-100"},
			{Name: "average_revs", Type: "int", Desc: "average revisions of the pair (ceil)"},
			{Name: "first_entity_revisions", Type: "int", Desc: "revisions of entity (--verbose only)"},
			{Name: "second_entity_revisions", Type: "int", Desc: "revisions of coupled (--verbose only)"},
			{Name: "shared_revisions", Type: "int", Desc: "revisions both changed in (--verbose only)"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runCoupling,
	}
}

// runCoupling scores every coupled entity pair: it collects each pair's shared
// and per-entity revision counts (dropping oversized change sets), derives the
// coupling degree and average revisions with code-maat's rounding, keeps only
// pairs clearing every threshold, and orders the survivors by degree then
// average-revs descending. Entity/coupled break ties ascending so the ordering
// is fully deterministic for --rows truncation.
func runCoupling(mods []model.Modification, opts Opts) (any, error) {
	thresholds := couplingalgo.Opts{
		MinRevs:       opts.MinRevs,
		MinSharedRevs: opts.MinSharedRevs,
		MinCoupling:   opts.MinCoupling,
		MaxCoupling:   opts.MaxCoupling,
	}

	pairs := couplingalgo.Couplings(mods, opts.MaxChangesetSize)

	rows := make([]couplingRow, 0, len(pairs))
	maxDegree := 0
	for _, p := range pairs {
		avg := calc.Average(p.EntityRevs, p.CoupledRevs)
		degree := calc.Percentage(float64(p.Shared) / avg)

		if d := calc.TruncInt(degree); d > maxDegree {
			maxDegree = d
		}

		// within-threshold? takes the average revisions as its revs argument;
		// floor(avg) equals the raw ratio for the inclusive >= min-revs check.
		if !couplingalgo.WithinThreshold(calc.TruncInt(avg), p.Shared, degree, thresholds) {
			continue
		}

		row := couplingRow{
			Entity:      p.Entity,
			Coupled:     p.Coupled,
			Degree:      calc.TruncInt(degree),
			AverageRevs: calc.Ceil(avg),
		}
		if opts.Verbose {
			entityRevs, coupledRevs, shared := p.EntityRevs, p.CoupledRevs, p.Shared
			row.FirstEntityRevisions = &entityRevs
			row.SecondEntityRevisions = &coupledRevs
			row.SharedRevisions = &shared
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Degree != rows[j].Degree {
			return rows[i].Degree > rows[j].Degree
		}
		if rows[i].AverageRevs != rows[j].AverageRevs {
			return rows[i].AverageRevs > rows[j].AverageRevs
		}
		if rows[i].Entity != rows[j].Entity {
			return rows[i].Entity < rows[j].Entity
		}
		return rows[i].Coupled < rows[j].Coupled
	})

	// Every candidate pair fell below the thresholds. The empty result is valid
	// but reads as "no coupling"; warn with the highest degree actually seen so
	// the operator can tell a threshold mismatch from a genuine absence.
	if len(rows) == 0 && len(pairs) > 0 {
		opts.warn(
			"coupling_all_filtered",
			"0 pairs met the coupling thresholds",
			fmt.Sprintf("highest observed coupling was %d%%; lower --min-coupling (currently %d) to see weaker links", maxDegree, opts.MinCoupling),
			map[string]any{"max_degree": maxDegree, "min_coupling": opts.MinCoupling, "candidate_pairs": len(pairs)},
		)
	}

	return rows, nil
}
