package analysis

import (
	"fmt"
	"sort"
	"strings"

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
		Shape:   ShapeTable,
		Flags: []Flag{
			{Name: "min-revs", Type: "int", Default: 5, Desc: "minimum revisions for a pair's average to be included"},
			{Name: "min-shared-revs", Type: "int", Default: 5, Desc: "minimum shared revisions for a pair"},
			{Name: "min-coupling", Type: "int", Default: 30, Desc: "minimum coupling degree in percent"},
			{Name: "max-coupling", Type: "int", Default: 100, Desc: "maximum coupling degree in percent"},
			{Name: "max-changeset-size", Type: "int", Default: 30, Desc: "skip change sets larger than this size"},
			{Name: "verbose", Type: "bool", Default: false, Desc: "add per-pair revision detail columns"},
		},
		RowSchema: []Column{
			{Name: "entity", Type: "string", Semantic: SemanticFilepath, Desc: "module path"},
			{Name: "coupled", Type: "string", Semantic: SemanticFilepath, Desc: "co-changing module path"},
			{Name: "degree", Type: "int", Semantic: SemanticPercentage, Desc: "coupling strength, percent 0-100"},
			{Name: "average_revs", Type: "int", Semantic: SemanticCount, Desc: "average revisions of the pair (ceil)"},
			{Name: "first_entity_revisions", Type: "int", Semantic: SemanticCount, Desc: "revisions of entity (--verbose only)", FlagGated: "verbose"},
			{Name: "second_entity_revisions", Type: "int", Semantic: SemanticCount, Desc: "revisions of coupled (--verbose only)", FlagGated: "verbose"},
			{Name: "shared_revisions", Type: "int", Semantic: SemanticCount, Desc: "revisions both changed in (--verbose only)", FlagGated: "verbose"},
		},
		ChangesetBased: true,
		ErrorCodes:     []string{"empty_log"},
		ExitCodes:      []int{0, 64, 65, 70, 74},
		Run:            runCoupling,
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
	var blockers couplingBlockers
	for _, p := range pairs {
		avg := calc.Average(p.EntityRevs, p.CoupledRevs)
		degree := calc.Percentage(float64(p.Shared) / avg)
		avgRevs := calc.TruncInt(avg)
		deg := calc.TruncInt(degree)

		if deg > blockers.maxDegree {
			blockers.maxDegree = deg
		}
		if p.Shared > blockers.maxSharedRevs {
			blockers.maxSharedRevs = p.Shared
		}
		if avgRevs > blockers.maxAverageRevs {
			blockers.maxAverageRevs = avgRevs
		}

		// within-threshold? takes the average revisions as its revs argument;
		// floor(avg) equals the raw ratio for the inclusive >= min-revs check.
		if !couplingalgo.WithinThreshold(avgRevs, p.Shared, degree, thresholds) {
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
	// but reads as "no coupling"; attribute it to the threshold(s) that actually
	// bound so the hint never sends a caller to a flag that excluded nothing.
	if len(rows) == 0 && len(pairs) > 0 {
		emitCouplingFilteredWarning(opts, blockers, len(pairs))
	}

	return rows, nil
}

// couplingBlockers records the best value observed for each bounded quantity
// across every candidate pair. Because the thresholds are a conjunction, a
// quantity whose best observation still fails its threshold is one that bound the
// whole result: naming it is what lets the caller lower the flag that actually
// excluded the closest pairs, rather than one the highest observed value already
// cleared.
type couplingBlockers struct {
	maxDegree      int
	maxSharedRevs  int
	maxAverageRevs int
}

// emitCouplingFilteredWarning raises coupling_all_filtered, attributing the empty
// result to the threshold(s) no candidate pair could clear. It names every binding
// threshold (a pair can fail several clauses at once) with its best observation so
// a caller can compute a working value, and carries a machine-readable `blocking`
// list so an agent can branch without parsing the prose hint.
func emitCouplingFilteredWarning(opts Opts, b couplingBlockers, candidatePairs int) {
	blocking := []string{}
	var actions []string
	if b.maxAverageRevs < opts.MinRevs {
		blocking = append(blocking, "min-revs")
		actions = append(actions, "lower --min-revs")
	}
	if b.maxSharedRevs < opts.MinSharedRevs {
		blocking = append(blocking, "min-shared-revs")
		actions = append(actions, "lower --min-shared-revs")
	}
	if b.maxDegree < opts.MinCoupling {
		blocking = append(blocking, "min-coupling")
		actions = append(actions, "lower --min-coupling")
	}
	if b.maxDegree > opts.MaxCoupling {
		blocking = append(blocking, "max-coupling")
		actions = append(actions, "raise --max-coupling")
	}

	hint := fmt.Sprintf(
		"no candidate pair cleared every threshold; best observed: average revs %d (--min-revs %d), shared revs %d (--min-shared-revs %d), degree %d%% (--min-coupling %d, --max-coupling %d)",
		b.maxAverageRevs, opts.MinRevs, b.maxSharedRevs, opts.MinSharedRevs, b.maxDegree, opts.MinCoupling, opts.MaxCoupling,
	)
	if len(actions) > 0 {
		hint += "; " + strings.Join(actions, " and ")
	}

	opts.warn(
		"coupling_all_filtered",
		"0 pairs met the coupling thresholds",
		hint,
		map[string]any{
			"candidate_pairs": candidatePairs,
			"blocking":        blocking,
			"observed": map[string]any{
				"max_degree":       b.maxDegree,
				"max_shared_revs":  b.maxSharedRevs,
				"max_average_revs": b.maxAverageRevs,
			},
			"thresholds": map[string]any{
				"min-revs":        opts.MinRevs,
				"min-shared-revs": opts.MinSharedRevs,
				"min-coupling":    opts.MinCoupling,
				"max-coupling":    opts.MaxCoupling,
			},
		},
	)
}
