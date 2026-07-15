package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// cmod is a terse constructor for the Modification fields the coupling analysis
// reads (entity and rev); all other fields are irrelevant here and left zero.
func cmod(entity, rev string) model.Modification {
	return model.Modification{Entity: entity, Rev: rev}
}

// couplingOpts returns the default coupling thresholds so cases can start from
// parity defaults and override only the field under test.
func couplingOpts() Opts {
	return Opts{
		MinRevs:          5,
		MinSharedRevs:    5,
		MinCoupling:      30,
		MaxCoupling:      100,
		MaxChangesetSize: 30,
	}
}

// couplingResultRows runs the analysis and returns its rows as the concrete row
// type, asserting the row type along the way.
func couplingResultRows(t *testing.T, mods []model.Modification, opts Opts) []couplingRow {
	t.Helper()
	rows, err := runCoupling(mods, opts)
	if err != nil {
		t.Fatalf("runCoupling returned error: %v", err)
	}
	got, ok := rows.([]couplingRow)
	if !ok {
		t.Fatalf("rows is %T, want []couplingRow", rows)
	}
	return got
}

// alwaysTogether builds a log where entities a and b change together in n
// distinct revisions, tagged with the given rev prefix so disjoint pairs never
// share a revision.
func alwaysTogether(prefix, a, b string, n int) []model.Modification {
	mods := make([]model.Modification, 0, n*2)
	for i := 1; i <= n; i++ {
		rev := prefix + string(rune('0'+i))
		mods = append(mods, cmod(a, rev), cmod(b, rev))
	}
	return mods
}

// TestCoupling_TwoModulesAlwaysTogether: two modules changed in lockstep across
// every revision are 100% coupled, with average-revs equal to their shared count.
func TestCoupling_TwoModulesAlwaysTogether(t *testing.T) {
	mods := alwaysTogether("r", "A", "B", 5)

	rows := couplingResultRows(t, mods, couplingOpts())

	want := []couplingRow{{Entity: "A", Coupled: "B", Degree: 100, AverageRevs: 5}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

// TestCoupling_DegreeAndRounding pins the degree and average-revs rounding: A
// changes in 8 revisions, B in 5, they share 5. average = 6.5, so degree =
// int(100*5/6.5) = int(76.92) = 76 (truncated) and average-revs = ceil(6.5) = 7.
func TestCoupling_DegreeAndRounding(t *testing.T) {
	mods := []model.Modification{
		cmod("A", "r1"), cmod("B", "r1"),
		cmod("A", "r2"), cmod("B", "r2"),
		cmod("A", "r3"), cmod("B", "r3"),
		cmod("A", "r4"), cmod("B", "r4"),
		cmod("A", "r5"), cmod("B", "r5"),
		cmod("A", "r6"),
		cmod("A", "r7"),
		cmod("A", "r8"),
	}

	rows := couplingResultRows(t, mods, couplingOpts())

	want := []couplingRow{{Entity: "A", Coupled: "B", Degree: 76, AverageRevs: 7}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

// TestCoupling_ThresholdFilters checks that a pair is excluded when its degree
// falls below --min-coupling or its floored degree exceeds --max-coupling, and
// included when both bounds are satisfied. The fixture's degree is 76.
func TestCoupling_ThresholdFilters(t *testing.T) {
	mods := []model.Modification{
		cmod("A", "r1"), cmod("B", "r1"),
		cmod("A", "r2"), cmod("B", "r2"),
		cmod("A", "r3"), cmod("B", "r3"),
		cmod("A", "r4"), cmod("B", "r4"),
		cmod("A", "r5"), cmod("B", "r5"),
		cmod("A", "r6"),
		cmod("A", "r7"),
		cmod("A", "r8"),
	}

	tests := []struct {
		name        string
		mutate      func(o Opts) Opts
		wantInclude bool
	}{
		{"included at defaults", func(o Opts) Opts { return o }, true},
		{"below min-coupling excluded", func(o Opts) Opts { o.MinCoupling = 80; return o }, false},
		{"above max-coupling excluded", func(o Opts) Opts { o.MaxCoupling = 75; return o }, false},
		{"below min-shared-revs excluded", func(o Opts) Opts { o.MinSharedRevs = 6; return o }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := couplingResultRows(t, mods, tt.mutate(couplingOpts()))
			if got := len(rows) == 1; got != tt.wantInclude {
				t.Errorf("included = %v, want %v (rows=%+v)", got, tt.wantInclude, rows)
			}
		})
	}
}

// TestCoupling_MaxChangesetSize verifies an oversized change set is dropped
// whole: with the fifth (A,B) revision above the size limit, the shared count
// falls to 4 and the pair no longer clears --min-shared-revs.
func TestCoupling_MaxChangesetSize(t *testing.T) {
	mods := []model.Modification{
		cmod("A", "r1"), cmod("B", "r1"),
		cmod("A", "r2"), cmod("B", "r2"),
		cmod("A", "r3"), cmod("B", "r3"),
		cmod("A", "r4"), cmod("B", "r4"),
		cmod("A", "r5"), cmod("B", "r5"), cmod("C", "r5"), cmod("D", "r5"), cmod("E", "r5"),
	}

	included := couplingResultRows(t, mods, couplingOpts())
	if len(included) != 1 || included[0].Degree != 100 {
		t.Fatalf("at default max-changeset-size, want one 100%% pair, got %+v", included)
	}

	opts := couplingOpts()
	opts.MaxChangesetSize = 4
	dropped := couplingResultRows(t, mods, opts)
	if len(dropped) != 0 {
		t.Errorf("with max-changeset-size 4, want empty (shared drops to 4), got %+v", dropped)
	}
}

// TestCoupling_Verbose adds the first/second/shared revision columns only when
// --verbose is set; without it those pointer columns are nil (and omitted).
func TestCoupling_Verbose(t *testing.T) {
	mods := alwaysTogether("r", "A", "B", 5)

	plain := couplingResultRows(t, mods, couplingOpts())
	if plain[0].FirstEntityRevisions != nil || plain[0].SecondEntityRevisions != nil || plain[0].SharedRevisions != nil {
		t.Errorf("non-verbose row carries revision columns: %+v", plain[0])
	}

	opts := couplingOpts()
	opts.Verbose = true
	verbose := couplingResultRows(t, mods, opts)
	got := verbose[0]
	if got.FirstEntityRevisions == nil || got.SecondEntityRevisions == nil || got.SharedRevisions == nil {
		t.Fatalf("verbose row missing revision columns: %+v", got)
	}
	if *got.FirstEntityRevisions != 5 || *got.SecondEntityRevisions != 5 || *got.SharedRevisions != 5 {
		t.Errorf("verbose revisions = (%d,%d,%d), want (5,5,5)",
			*got.FirstEntityRevisions, *got.SecondEntityRevisions, *got.SharedRevisions)
	}
}

// TestCoupling_SortDesc verifies the ordering: degree descending, then
// average-revs descending, then entity/coupled ascending as the deterministic
// tiebreak. G,H (100, avg 6) precede A,B and E,F (both 100, avg 5), and A,B
// precedes E,F on entity name.
func TestCoupling_SortDesc(t *testing.T) {
	var mods []model.Modification
	mods = append(mods, alwaysTogether("g", "G", "H", 6)...)
	mods = append(mods, alwaysTogether("a", "A", "B", 5)...)
	mods = append(mods, alwaysTogether("e", "E", "F", 5)...)

	rows := couplingResultRows(t, mods, couplingOpts())

	want := []couplingRow{
		{Entity: "G", Coupled: "H", Degree: 100, AverageRevs: 6},
		{Entity: "A", Coupled: "B", Degree: 100, AverageRevs: 5},
		{Entity: "E", Coupled: "F", Degree: 100, AverageRevs: 5},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

// TestCoupling_Empty: an empty log is a valid, empty result (not an error).
func TestCoupling_Empty(t *testing.T) {
	rows := couplingResultRows(t, nil, couplingOpts())
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

// TestCoupling_DescriptorRegistered pins the descriptor's identity, standard
// columns, and the coupling threshold flags with their default values.
func TestCoupling_DescriptorRegistered(t *testing.T) {
	d := couplingDescriptor()
	if d.Name != "coupling" {
		t.Errorf("Name = %q, want %q", d.Name, "coupling")
	}
	wantCols := []string{"entity", "coupled", "degree", "average_revs"}
	for i, name := range wantCols {
		if d.RowSchema[i].Name != name {
			t.Errorf("RowSchema[%d].Name = %q, want %q", i, d.RowSchema[i].Name, name)
		}
	}
	wantFlags := map[string]any{
		"min-revs":           5,
		"min-shared-revs":    5,
		"min-coupling":       30,
		"max-coupling":       100,
		"max-changeset-size": 30,
		"verbose":            false,
	}
	got := map[string]any{}
	for _, f := range d.Flags {
		got[f.Name] = f.Default
	}
	if !reflect.DeepEqual(got, wantFlags) {
		t.Errorf("flags/defaults = %+v, want %+v", got, wantFlags)
	}
	if d.Run == nil {
		t.Error("Run is nil")
	}
}
