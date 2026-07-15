// Package couplingalgo holds the core logical-coupling algorithms shared by the
// coupling and sum-of-coupling analyses. It ports code-maat's coupling_algos.clj
// faithfully, keeping the two load-bearing subtleties in one place: self-pairs
// are retained when totalling per-module revisions (so a singleton change set
// still counts its module) but dropped when counting shared revisions per real
// pair.
package couplingalgo

import (
	"math"
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
)

// Opts carries the tuning thresholds the coupling threshold filter reads. It is
// deliberately a small local type rather than the analysis package's Opts: this
// package is imported by analysis, so importing it back would form a cycle. The
// coupling analysis populates these fields from its effective options.
type Opts struct {
	// MinRevs is the minimum revisions a pair's average must reach to be kept.
	MinRevs int
	// MinSharedRevs is the minimum shared revisions a pair must have.
	MinSharedRevs int
	// MinCoupling is the inclusive lower bound on the coupling degree (percent).
	MinCoupling int
	// MaxCoupling is the inclusive upper bound on the floored coupling degree.
	MaxCoupling int
}

// pair is an unordered pair of entities held in a canonical, sorted form: A is
// never greater than B. A self-pair has A == B. Using a comparable struct lets
// pairs be map keys for the frequency counts.
type pair struct {
	A string
	B string
}

// newPair returns the canonical (sorted) pair of x and y, so that {x,y} and
// {y,x} compare equal.
func newPair(x, y string) pair {
	if x > y {
		x, y = y, x
	}
	return pair{A: x, B: y}
}

// changeSetsByRevision groups modifications into one change set per revision:
// the distinct entities touched by that revision. Revisions come back in
// ascending key order (via calc.GroupBy) and, within a set, entities keep their
// first-seen order, so the result is deterministic across runs.
func changeSetsByRevision(mods []model.Modification) [][]string {
	groups := calc.GroupBy(mods, func(m model.Modification) string { return m.Rev })
	sets := make([][]string, 0, len(groups))
	for _, g := range groups {
		entities := make([]string, 0, len(g.Items))
		for _, m := range g.Items {
			entities = append(entities, m.Entity)
		}
		sets = append(sets, calc.Distinct(entities))
	}
	return sets
}

// coChangingByRevision expands each change set into its co-changing pairs. Sets
// larger than maxChangesetSize are dropped whole (matching --max-changeset-size).
// For a kept set it forms the selections-with-replacement of size two, canonicalizes
// each to a sorted pair, then sorts and de-duplicates, so {A,B} yields the three
// pairs {A,A}, {A,B}, {B,B}. The self-pairs are retained here on purpose; they are
// what keeps a module's own revisions countable in moduleByRevs.
func coChangingByRevision(sets [][]string, maxChangesetSize int) [][]pair {
	out := make([][]pair, 0, len(sets))
	for _, set := range sets {
		if len(set) > maxChangesetSize {
			continue
		}
		pairs := make([]pair, 0, len(set)*len(set))
		for _, x := range set {
			for _, y := range set {
				pairs = append(pairs, newPair(x, y))
			}
		}
		out = append(out, sortDistinctPairs(pairs))
	}
	return out
}

// sortDistinctPairs sorts pairs by (A, B) and removes duplicates, giving the
// canonical pair list for one change set.
func sortDistinctPairs(pairs []pair) []pair {
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].A != pairs[j].A {
			return pairs[i].A < pairs[j].A
		}
		return pairs[i].B < pairs[j].B
	})
	return calc.Distinct(pairs)
}

// moduleByRevs counts, per module, the number of revisions it participated in.
// For each revision it collects the distinct modules present across that
// revision's pairs (both sides of every pair) and increments each one. Self-pairs
// ensure a module changed alone in a revision is still counted.
func moduleByRevs(coChanging [][]pair) map[string]int {
	revs := make(map[string]int)
	for _, pairs := range coChanging {
		modules := make([]string, 0, len(pairs)*2)
		for _, p := range pairs {
			modules = append(modules, p.A, p.B)
		}
		for _, m := range calc.Distinct(modules) {
			revs[m]++
		}
	}
	return revs
}

// couplingFrequencies counts, per real (cross-entity) pair, the number of
// revisions in which the two entities co-changed. Self-pairs are dropped here,
// so the result holds only genuine coupling relationships; each pair's value is
// its shared-revision count.
func couplingFrequencies(coChanging [][]pair) map[pair]int {
	freq := make(map[pair]int)
	for _, pairs := range coChanging {
		for _, p := range pairs {
			if p.A == p.B {
				continue
			}
			freq[p]++
		}
	}
	return freq
}

// PairRevs is one coupled (cross-entity) pair together with the revision counts
// the coupling analysis needs to score it: the shared revisions in which both
// entities changed, and each entity's own total revisions. Entity is the
// alphabetically-smaller side of the canonical pair and Coupled the larger, so
// each unordered pair appears once.
type PairRevs struct {
	// Entity is the first (sorted) entity of the pair.
	Entity string
	// Coupled is the second (sorted) entity of the pair.
	Coupled string
	// Shared is the number of revisions in which both entities co-changed.
	Shared int
	// EntityRevs is the total revisions the first entity participated in.
	EntityRevs int
	// CoupledRevs is the total revisions the second entity participated in.
	CoupledRevs int
}

// Couplings computes the per-pair coupling statistics for mods: it forms the
// per-revision change sets, drops any larger than maxChangesetSize, then counts
// each real pair's shared revisions and each module's own revisions (self-pairs
// keep a module changed alone countable). The returned pairs are sorted by
// (Entity, Coupled) ascending so the coupling analysis's own degree-ordered sort
// and any --rows truncation are deterministic. Threshold filtering and rounding
// are the caller's concern; this returns the raw revision counts for every pair.
func Couplings(mods []model.Modification, maxChangesetSize int) []PairRevs {
	coChanging := coChangingByRevision(changeSetsByRevision(mods), maxChangesetSize)
	moduleRevs := moduleByRevs(coChanging)
	freqs := couplingFrequencies(coChanging)

	out := make([]PairRevs, 0, len(freqs))
	for p, shared := range freqs {
		out = append(out, PairRevs{
			Entity:      p.A,
			Coupled:     p.B,
			Shared:      shared,
			EntityRevs:  moduleRevs[p.A],
			CoupledRevs: moduleRevs[p.B],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Entity != out[j].Entity {
			return out[i].Entity < out[j].Entity
		}
		return out[i].Coupled < out[j].Coupled
	})
	return out
}

// WithinThreshold reports whether a coupled pair clears every tuning threshold:
// its average revisions and shared revisions meet their inclusive minimums, its
// coupling degree meets the inclusive minimum, and the floored degree stays
// within the inclusive maximum. This mirrors code-maat's within-threshold?.
func WithinThreshold(revs, sharedRevs int, coupling float64, o Opts) bool {
	return revs >= o.MinRevs &&
		sharedRevs >= o.MinSharedRevs &&
		coupling >= float64(o.MinCoupling) &&
		math.Floor(coupling) <= float64(o.MaxCoupling)
}
