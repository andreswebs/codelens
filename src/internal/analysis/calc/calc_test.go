package calc

import (
	"reflect"
	"testing"
)

// TestAverage pins the arithmetic mean used by coupling's average-revs.
func TestAverage(t *testing.T) {
	if got := Average(44, 45); got != 44.5 {
		t.Errorf("Average(44, 45) = %v, want 44.5", got)
	}
	if got := Average(10, 10); got != 10 {
		t.Errorf("Average(10, 10) = %v, want 10", got)
	}
}

// TestPercentage_TruncInt_Degree reproduces the coupling degree computation:
// a shared/average ratio expressed as a percentage and truncated to an int.
func TestPercentage_TruncInt_Degree(t *testing.T) {
	avg := Average(44, 45) // 44.5
	shared := 35
	pct := Percentage(float64(shared) / avg)
	if pct < 78.6 || pct > 78.7 {
		t.Errorf("Percentage(35/44.5) = %v, want ~78.65", pct)
	}
	if got := TruncInt(pct); got != 78 {
		t.Errorf("TruncInt(%v) = %d, want 78", pct, got)
	}
}

// TestCeil_AverageRevs pins the ceil applied to average-revs output.
func TestCeil_AverageRevs(t *testing.T) {
	if got := Ceil(44.5); got != 45 {
		t.Errorf("Ceil(44.5) = %d, want 45", got)
	}
	if got := Ceil(44.0); got != 44 {
		t.Errorf("Ceil(44.0) = %d, want 44", got)
	}
}

// TestTruncInt_TowardZero documents truncation toward zero, matching the
// original's int(...) semantics.
func TestTruncInt_TowardZero(t *testing.T) {
	cases := []struct {
		in   float64
		want int
	}{
		{78.99, 78},
		{78.01, 78},
		{0.0, 0},
		{-1.9, -1},
	}
	for _, c := range cases {
		if got := TruncInt(c.in); got != c.want {
			t.Errorf("TruncInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestCentiRatio_TwoSigDigits pins ownership rounding to 2 significant digits,
// reproducing the original's ratio->centi-float-precision exactly.
func TestCentiRatio_TwoSigDigits(t *testing.T) {
	cases := []struct {
		own, total int
		want       float64
	}{
		{834, 1000, 0.83},
		{834, 10000, 0.083},
		{1, 3, 0.33},
		{5, 0, 5.0},
		{5, 1, 5.0},
	}
	for _, c := range cases {
		if got := CentiRatio(c.own, c.total); got != c.want {
			t.Errorf("CentiRatio(%d, %d) = %v, want %v", c.own, c.total, got, c.want)
		}
	}
}

// TestCentiFloat_TwoSigDigits pins the standalone 2-significant-digit rounding
// used by analyses (fragmentation) that compute their float value directly
// rather than as an own/total ratio.
func TestCentiFloat_TwoSigDigits(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0.0, 0.0},
		{0.5, 0.5},
		{0.834, 0.83},
		{0.0834, 0.083},
		{0.6666666666666667, 0.67},
	}
	for _, c := range cases {
		if got := CentiFloat(c.in); got != c.want {
			t.Errorf("CentiFloat(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestGroupBy_Deterministic verifies groups come back keyed in ascending key
// order, so downstream sorting and truncation are stable across runs.
func TestGroupBy_Deterministic(t *testing.T) {
	xs := []string{"b", "a", "b", "c", "a"}
	groups := GroupBy(xs, func(s string) string { return s })

	var keys []string
	byKey := map[string][]string{}
	for _, g := range groups {
		keys = append(keys, g.Key)
		byKey[g.Key] = g.Items
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("keys = %v, want %v", keys, want)
	}
	if want := []string{"a", "a"}; !reflect.DeepEqual(byKey["a"], want) {
		t.Errorf("group a = %v, want %v", byKey["a"], want)
	}
	if want := []string{"b", "b"}; !reflect.DeepEqual(byKey["b"], want) {
		t.Errorf("group b = %v, want %v", byKey["b"], want)
	}
	if want := []string{"c"}; !reflect.DeepEqual(byKey["c"], want) {
		t.Errorf("group c = %v, want %v", byKey["c"], want)
	}
}

// TestGroupBy_Empty verifies an empty input yields no groups (not a nil-index
// panic) so callers can range freely.
func TestGroupBy_Empty(t *testing.T) {
	if got := GroupBy([]string{}, func(s string) string { return s }); len(got) != 0 {
		t.Errorf("GroupBy(empty) = %v, want no groups", got)
	}
}

// TestDistinct_PreservesFirstSeen documents that Distinct keeps the first
// occurrence order of each value.
func TestDistinct_PreservesFirstSeen(t *testing.T) {
	xs := []string{"b", "a", "b", "c", "a"}
	if got, want := Distinct(xs), []string{"b", "a", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Distinct(%v) = %v, want %v", xs, got, want)
	}
}

// TestMaxBy_PicksMaxAndSums verifies MaxBy returns the greatest element by
// val and the sum of val over all elements.
func TestMaxBy_PicksMaxAndSums(t *testing.T) {
	items := []int{3, 7, 2, 5}
	top, total := MaxBy(items, func(x int) int { return x })
	if top != 7 {
		t.Errorf("top = %d, want 7", top)
	}
	if total != 17 {
		t.Errorf("total = %d, want 17", total)
	}
}

// TestMaxBy_TieKeepsFirst verifies ties resolve to the earliest element (strict
// greater-than), so a caller that pre-sorts its input gets a deterministic
// winner. Elements carry an index so we can tell which one was kept.
func TestMaxBy_TieKeepsFirst(t *testing.T) {
	type contrib struct {
		id  int
		val int
	}
	items := []contrib{{id: 0, val: 5}, {id: 1, val: 5}, {id: 2, val: 5}}
	top, total := MaxBy(items, func(c contrib) int { return c.val })
	if top.id != 0 {
		t.Errorf("top.id = %d, want 0 (first on tie)", top.id)
	}
	if total != 15 {
		t.Errorf("total = %d, want 15", total)
	}
}

// TestMaxBy_SingleElement verifies a one-element slice returns that element and
// its value as the total.
func TestMaxBy_SingleElement(t *testing.T) {
	top, total := MaxBy([]int{9}, func(x int) int { return x })
	if top != 9 || total != 9 {
		t.Errorf("MaxBy([9]) = (%d, %d), want (9, 9)", top, total)
	}
}

// TestMaxBy_NegativeAndZero verifies MaxBy handles zero and negative values:
// the greatest (closest to positive) wins and the total is the signed sum.
func TestMaxBy_NegativeAndZero(t *testing.T) {
	items := []int{-3, 0, -1, -5}
	top, total := MaxBy(items, func(x int) int { return x })
	if top != 0 {
		t.Errorf("top = %d, want 0", top)
	}
	if total != -9 {
		t.Errorf("total = %d, want -9", total)
	}
}

// TestMap_PreservesOrder verifies Map applies f to each element in order.
func TestMap_PreservesOrder(t *testing.T) {
	got := Map([]int{1, 2, 3}, func(x int) int { return x * 10 })
	if want := []int{10, 20, 30}; !reflect.DeepEqual(got, want) {
		t.Errorf("Map = %v, want %v", got, want)
	}
}

// TestMap_EmptyNonNil verifies Map on empty input returns a non-nil empty slice,
// matching the make([]R, 0, ...) the refactored loops replace (so JSON marshals
// to [] not null).
func TestMap_EmptyNonNil(t *testing.T) {
	got := Map([]int{}, func(x int) int { return x })
	if got == nil {
		t.Fatal("Map(empty) = nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("Map(empty) len = %d, want 0", len(got))
	}
}

// TestFlatMap_ConcatenatesInOrder verifies FlatMap concatenates f's outputs in
// element order.
func TestFlatMap_ConcatenatesInOrder(t *testing.T) {
	got := FlatMap([]int{1, 2, 3}, func(x int) []int { return []int{x, x} })
	if want := []int{1, 1, 2, 2, 3, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("FlatMap = %v, want %v", got, want)
	}
}

// TestFlatMap_EmptyNonNil verifies FlatMap on empty input returns a non-nil
// empty slice (JSON marshals to [] not null).
func TestFlatMap_EmptyNonNil(t *testing.T) {
	got := FlatMap([]int{}, func(x int) []int { return []int{x} })
	if got == nil {
		t.Fatal("FlatMap(empty) = nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("FlatMap(empty) len = %d, want 0", len(got))
	}
}

// TestFlatMap_NestedMap exercises the flatten-pair shape: FlatMap over entities
// whose inner rows come from Map, verifying the nested composition preserves the
// entity-then-author order the analyses rely on.
func TestFlatMap_NestedMap(t *testing.T) {
	type entity struct {
		name    string
		authors []string
	}
	entities := []entity{
		{name: "a", authors: []string{"x", "y"}},
		{name: "b", authors: []string{"z"}},
	}
	got := FlatMap(entities, func(e entity) []string {
		return Map(e.authors, func(a string) string { return e.name + ":" + a })
	})
	want := []string{"a:x", "a:y", "b:z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FlatMap(Map) = %v, want %v", got, want)
	}
}
