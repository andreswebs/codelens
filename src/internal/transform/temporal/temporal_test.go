package temporal_test

import (
	"errors"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/andreswebs/codelens/internal/transform/temporal"
)

// pair is a compact (entity, rev) view of a Modification used to assert window
// membership without over-specifying the untouched fields.
type pair struct {
	entity string
	rev    string
}

func pairsOf(mods []model.Modification) []pair {
	ps := make([]pair, len(mods))
	for i, m := range mods {
		ps[i] = pair{m.Entity, m.Rev}
	}
	return ps
}

func contains(ps []pair, want pair) bool {
	for _, p := range ps {
		if p == want {
			return true
		}
	}
	return false
}

func assertCoded(t *testing.T, err error, code string, exit int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error %v is not a terr.Coded", err)
	}
	if coded.Code() != code {
		t.Errorf("code = %q, want %q", coded.Code(), code)
	}
	if coded.ExitCode() != exit {
		t.Errorf("exit = %d, want %d", coded.ExitCode(), exit)
	}
}

func TestApply_InvalidPeriod(t *testing.T) {
	mods := []model.Modification{{Entity: "a", Rev: "1", Date: "2015-01-01"}}
	for _, period := range []int{0, -1, -30} {
		_, err := temporal.Apply(mods, period)
		assertCoded(t, err, "invalid_temporal_period", 2)
	}
}

func TestApply_Empty(t *testing.T) {
	out, err := temporal.Apply(nil, 1)
	if err != nil {
		t.Fatalf("temporal.Apply(nil, 1) returned error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("got %d rows, want 0", len(out))
	}
}

func TestApply_Period1_IsPerDayGrouping(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a", Rev: "1", Date: "2015-01-01", Author: "x"},
		{Entity: "b", Rev: "1", Date: "2015-01-01", Author: "x"},
		{Entity: "a", Rev: "2", Date: "2015-01-01", Author: "y"},
	}
	out, err := temporal.Apply(mods, 1)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d rows, want 2 (deduped by entity): %+v", len(out), out)
	}
	for _, m := range out {
		if m.Rev != "2015-01-01" {
			t.Errorf("entity %q rev = %q, want the day 2015-01-01", m.Entity, m.Rev)
		}
	}
	if out[0].Entity != "a" || out[0].Author != "x" {
		t.Errorf("first row = %+v, want entity a from the first occurrence (author x)", out[0])
	}
	if out[1].Entity != "b" {
		t.Errorf("second row entity = %q, want b", out[1].Entity)
	}
}

func TestApply_Period2_SlidingOverlap(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a", Rev: "1", Date: "2015-01-01"},
		{Entity: "b", Rev: "2", Date: "2015-01-02"},
		{Entity: "c", Rev: "3", Date: "2015-01-03"},
	}
	out, err := temporal.Apply(mods, 2)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	// Two overlapping windows: [01,02]=>rev 01-02, [02,03]=>rev 01-03.
	if len(out) != 4 {
		t.Fatalf("got %d rows, want 4: %+v", len(out), out)
	}
	ps := pairsOf(out)
	want := []pair{
		{"a", "2015-01-02"},
		{"b", "2015-01-02"},
		{"b", "2015-01-03"},
		{"c", "2015-01-03"},
	}
	for _, w := range want {
		if !contains(ps, w) {
			t.Errorf("missing window record %+v in %+v", w, ps)
		}
	}
}

func TestApply_DedupeByEntity(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a", Rev: "1", Date: "2015-01-01"},
		{Entity: "a", Rev: "2", Date: "2015-01-02"},
	}
	out, err := temporal.Apply(mods, 2)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1 (same entity deduped in one window): %+v", len(out), out)
	}
	if out[0].Entity != "a" || out[0].Rev != "2015-01-02" {
		t.Errorf("row = %+v, want entity a with rev 2015-01-02 (window latest)", out[0])
	}
}

// TestApply_PadsAndSkipsEmptyWindows exercises the distinctive behaviors: the
// day range is padded so a window's latest calendar day becomes the rev even
// when nothing changed that day, and windows spanning only empty days are
// dropped. Ported semantics from time_based_grouper_test.clj (the upstream
// corpus symlink is not available in this environment).
func TestApply_PadsAndSkipsEmptyWindows(t *testing.T) {
	mods := []model.Modification{
		{Entity: "a", Rev: "1", Date: "2015-01-01"},
		{Entity: "b", Rev: "2", Date: "2015-01-05"},
	}
	out, err := temporal.Apply(mods, 2)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	// Padded days 01..05 -> windows [01,02][02,03][03,04][04,05]; the middle
	// two are empty and skipped.
	if len(out) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(out), out)
	}
	if out[0].Entity != "a" || out[0].Rev != "2015-01-02" {
		t.Errorf("row 0 = %+v, want entity a rev 2015-01-02 (padded window latest)", out[0])
	}
	if out[1].Entity != "b" || out[1].Rev != "2015-01-05" {
		t.Errorf("row 1 = %+v, want entity b rev 2015-01-05", out[1])
	}
}

// TestApply_PeriodExceedsSpan documents the faithful sliding-window semantics
// (Clojure `partition n 1` drops an incomplete trailing window): when the whole
// day range is shorter than the period, no complete window exists and the result
// is empty.
func TestApply_PeriodExceedsSpan(t *testing.T) {
	mods := []model.Modification{{Entity: "a", Rev: "1", Date: "2015-01-01"}}
	out, err := temporal.Apply(mods, 2)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("got %d rows, want 0 (range shorter than the period): %+v", len(out), out)
	}
}

func TestApply_InvalidDate(t *testing.T) {
	mods := []model.Modification{{Entity: "a", Rev: "1", Date: "not-a-date"}}
	_, err := temporal.Apply(mods, 1)
	assertCoded(t, err, "invalid_temporal_date", 3)
}
