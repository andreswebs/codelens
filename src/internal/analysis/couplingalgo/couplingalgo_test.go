package couplingalgo

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// TestChangeSets_GroupByRev verifies that modifications are grouped into one
// distinct-entity change set per revision, in ascending revision order, with
// entity order preserved and duplicates within a revision removed.
func TestChangeSets_GroupByRev(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a.go", Rev: "r1"},
		{Entity: "b.go", Rev: "r1"},
		{Entity: "a.go", Rev: "r1"},
		{Entity: "c.go", Rev: "r2"},
	}

	got := changeSetsByRevision(mods)

	want := [][]string{
		{"a.go", "b.go"},
		{"c.go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changeSetsByRevision() = %v, want %v", got, want)
	}
}

// TestCoChanging_PairsIncludeSelf verifies that a change set is expanded into
// unordered, sorted pairs that include the self-pairs, e.g. {A,B} yields
// {A,A}, {A,B}, {B,B}. Self-pairs are what let a module's own revisions be
// counted even in a singleton change set.
func TestCoChanging_PairsIncludeSelf(t *testing.T) {
	sets := [][]string{{"A", "B"}}

	got := coChangingByRevision(sets, 30)

	want := [][]pair{{
		{A: "A", B: "A"},
		{A: "A", B: "B"},
		{A: "B", B: "B"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("coChangingByRevision() = %v, want %v", got, want)
	}
}

// TestModuleByRevs verifies that each module is counted once per revision it
// participates in. Because self-pairs are present, a module changed alone in a
// revision (a singleton change set) is still counted for that revision.
func TestModuleByRevs(t *testing.T) {
	coChanging := [][]pair{
		{{A: "A", B: "A"}, {A: "A", B: "B"}, {A: "B", B: "B"}},
		{{A: "A", B: "A"}},
	}

	got := moduleByRevs(coChanging)

	want := map[string]int{"A": 2, "B": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("moduleByRevs() = %v, want %v", got, want)
	}
}

// TestCouplingFrequencies_DropsSelfPairs verifies that shared-revision counts
// are computed only over real (cross-entity) pairs: self-pairs are dropped, and
// each real pair's count is the number of revisions in which it co-changed.
func TestCouplingFrequencies_DropsSelfPairs(t *testing.T) {
	coChanging := [][]pair{
		{{A: "A", B: "A"}, {A: "A", B: "B"}, {A: "B", B: "B"}},
		{{A: "A", B: "A"}},
		{{A: "A", B: "A"}, {A: "A", B: "B"}, {A: "B", B: "B"}},
	}

	got := couplingFrequencies(coChanging)

	want := map[pair]int{{A: "A", B: "B"}: 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("couplingFrequencies() = %v, want %v", got, want)
	}
}

// TestWithinThreshold_Bounds checks each threshold at its boundary: revs and
// shared-revs are inclusive lower bounds, min-coupling is an inclusive lower
// bound on the degree, and max-coupling is an inclusive upper bound on the
// floored degree.
func TestWithinThreshold_Bounds(t *testing.T) {
	base := Opts{MinRevs: 5, MinSharedRevs: 5, MinCoupling: 30, MaxCoupling: 50}

	tests := []struct {
		name     string
		revs     int
		shared   int
		coupling float64
		opts     Opts
		want     bool
	}{
		{"all at lower bounds", 5, 5, 30, base, true},
		{"revs below min", 4, 5, 30, base, false},
		{"shared below min", 5, 4, 30, base, false},
		{"coupling below min", 5, 5, 29.9, base, false},
		{"coupling at min", 5, 5, 30.0, base, true},
		{"floor at max", 5, 5, 50.9, base, true},
		{"floor above max", 5, 5, 51.0, base, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithinThreshold(tt.revs, tt.shared, tt.coupling, tt.opts); got != tt.want {
				t.Errorf("WithinThreshold(%d, %d, %v) = %v, want %v",
					tt.revs, tt.shared, tt.coupling, got, tt.want)
			}
		})
	}
}

// TestCoChanging_DropsOversized verifies that change sets larger than
// maxChangesetSize are excluded entirely, while sets at or below the limit are
// kept.
func TestCoChanging_DropsOversized(t *testing.T) {
	sets := [][]string{
		{"A", "B", "C"},
		{"X", "Y"},
	}

	got := coChangingByRevision(sets, 2)

	want := [][]pair{{
		{A: "X", B: "X"},
		{A: "X", B: "Y"},
		{A: "Y", B: "Y"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("coChangingByRevision() = %v, want %v", got, want)
	}
}

// TestCouplings_ComputesPairStats verifies the exported Couplings entry point:
// it assembles, per real cross-entity pair, the shared-revision count and each
// entity's own revision total, applies the max-changeset-size drop, and returns
// the pairs sorted by (entity, coupled). Here A changes in 5 revisions, B in 4,
// they co-change in 4, and a 3-entity revision is dropped by size 2.
func TestCouplings_ComputesPairStats(t *testing.T) {
	mods := []model.Modification{
		{Entity: "A", Rev: "r1"}, {Entity: "B", Rev: "r1"},
		{Entity: "A", Rev: "r2"}, {Entity: "B", Rev: "r2"},
		{Entity: "A", Rev: "r3"}, {Entity: "B", Rev: "r3"},
		{Entity: "A", Rev: "r4"}, {Entity: "B", Rev: "r4"},
		{Entity: "A", Rev: "r5"},
		{Entity: "X", Rev: "r6"}, {Entity: "Y", Rev: "r6"}, {Entity: "Z", Rev: "r6"},
	}

	got := Couplings(mods, 2)

	want := []PairRevs{
		{Entity: "A", Coupled: "B", Shared: 4, EntityRevs: 5, CoupledRevs: 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Couplings() = %+v, want %+v", got, want)
	}
}

// TestCouplings_SortedByPair verifies the returned pairs are ordered by entity
// then coupled ascending, independent of revision iteration order.
func TestCouplings_SortedByPair(t *testing.T) {
	mods := []model.Modification{
		{Entity: "M", Rev: "r1"}, {Entity: "N", Rev: "r1"},
		{Entity: "A", Rev: "r2"}, {Entity: "Z", Rev: "r2"},
		{Entity: "A", Rev: "r3"}, {Entity: "B", Rev: "r3"},
	}

	got := Couplings(mods, 30)

	var order [][2]string
	for _, p := range got {
		order = append(order, [2]string{p.Entity, p.Coupled})
	}
	want := [][2]string{{"A", "B"}, {"A", "Z"}, {"M", "N"}}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("pair order = %v, want %v", order, want)
	}
}
