// Package calc holds the shared aggregation and rounding helpers used by every
// analysis. The rounding rules are load-bearing for numeric parity with
// code-maat, so they live in one place and are pinned by tests: average-revs is
// ceil(average), coupling degree is int(percentage) (truncation toward zero),
// and ownership is rounded to two significant digits (CentiRatio), reproducing
// the original's ratio->centi-float-precision.
package calc

import (
	"math"
	"sort"
	"strconv"
)

// Group is one bucket produced by GroupBy: a key and the input elements that
// mapped to it, in their original order.
type Group[T any] struct {
	// Key is the value returned by the grouping function.
	Key string
	// Items are the elements that share this key, in first-seen order.
	Items []T
}

// GroupBy partitions xs by the key function and returns the groups in ascending
// key order. Ordering the groups by key (rather than by first-seen key) makes
// downstream sorting and --rows truncation deterministic across runs, which the
// original leaves to dataset insertion order. Within a group the elements keep
// their original relative order.
func GroupBy[T any](xs []T, key func(T) string) []Group[T] {
	index := map[string][]T{}
	var keys []string
	for _, x := range xs {
		k := key(x)
		if _, seen := index[k]; !seen {
			keys = append(keys, k)
		}
		index[k] = append(index[k], x)
	}
	sort.Strings(keys)

	groups := make([]Group[T], 0, len(keys))
	for _, k := range keys {
		groups = append(groups, Group[T]{Key: k, Items: index[k]})
	}
	return groups
}

// Distinct returns the unique values of xs, preserving the order of first
// appearance.
func Distinct[T comparable](xs []T) []T {
	seen := make(map[T]struct{}, len(xs))
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

// MaxBy returns the first element of items with the greatest val(item) and the
// sum of val over all items. "First" means ties resolve to the earliest
// element (strict greater-than), so a caller that pre-sorts items (e.g.
// ascending author) gets a deterministic winner. items must be non-empty;
// every analysis calls it per entity, which always has at least one
// contributor.
func MaxBy[T any](items []T, val func(T) int) (top T, total int) {
	top = items[0]
	for _, item := range items {
		v := val(item)
		total += v
		if v > val(top) {
			top = item
		}
	}
	return top, total
}

// Map returns f applied to each element of src, preserving order. The result is
// always non-nil (an empty src yields an empty, non-nil slice), matching the
// make([]R, 0, ...) accumulation it replaces.
func Map[S, R any](src []S, f func(S) R) []R {
	out := make([]R, 0, len(src))
	for _, s := range src {
		out = append(out, f(s))
	}
	return out
}

// FlatMap returns the concatenation of f applied to each element of src,
// preserving order. Like Map, the result is always non-nil.
func FlatMap[S, R any](src []S, f func(S) []R) []R {
	out := make([]R, 0, len(src))
	for _, s := range src {
		out = append(out, f(s)...)
	}
	return out
}

// Average returns the arithmetic mean of a and b as a float64, matching the
// original's average(a, b) = (a + b) / 2.
func Average(a, b int) float64 {
	return float64(a+b) / 2
}

// Percentage scales a ratio to a percentage, matching as-percentage(v) = v*100.
func Percentage(v float64) float64 {
	return v * 100
}

// TruncInt truncates v toward zero, matching the original's int(...) semantics
// used for the coupling degree.
func TruncInt(v float64) int {
	return int(math.Trunc(v))
}

// Ceil rounds v up to the nearest integer, matching the ceil applied to
// average-revs.
func Ceil(v float64) int {
	return int(math.Ceil(v))
}

// CentiFloat rounds v to two significant digits, reproducing the original's
// ratio->centi-float-precision. Rounding is to significant digits, not decimal
// places: 0.834 -> 0.83, 0.0834 -> 0.083. Analyses whose value is already a
// computed float (fragmentation's fractal value) round through this directly;
// CentiRatio is the own/total convenience wrapper.
func CentiFloat(v float64) float64 {
	// 'g' with precision 2 formats to two significant digits; parsing back
	// yields the rounded float value.
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 2, 64), 64)
	if err != nil {
		// FormatFloat always emits a parseable number, so this is unreachable;
		// fall back to the unrounded value rather than discarding the error.
		return v
	}
	return rounded
}

// CentiRatio returns own/total rounded to two significant digits, reproducing
// the original's ratio->centi-float-precision used for ownership. A zero (or
// negative) total is treated as 1, matching the original's guard, so the ratio
// never divides by zero.
func CentiRatio(own, total int) float64 {
	denom := total
	if denom < 1 {
		denom = 1
	}
	return CentiFloat(float64(own) / float64(denom))
}
