// Package filter implements the path-filter transform: it keeps only the
// modifications whose entity matches the include/exclude glob rules. It runs
// first in the pipeline, before grouping, so the globs match raw file paths
// (e.g. `**/Migrations/**`) rather than the layer names grouping produces.
//
// Precedence is exclude-after-include: if any include is given, an entity must
// match at least one include to survive; then any exclude match drops it. With
// no includes, every entity is included and only excludes apply.
package filter

import (
	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/bmatcuk/doublestar/v4"
)

// maxPatternLen bounds a single glob's length. doublestar's matcher is
// backtracking-safe, so a length cap is the only guard needed against a
// pathological pattern, mirroring the group transform's maxPatternLen.
const maxPatternLen = 1000

// ErrInvalidGlob marks a malformed --include/--exclude glob: an empty,
// oversized, or syntactically invalid pattern. It is a usage error (exit 2)
// because globs are supplied directly via flags, never derived from the
// analyzed log. Callers surface the offending pattern via WithDetails.
var ErrInvalidGlob = terr.New(
	"invalid_glob", 2,
	"use gitignore-style globs against the full path, e.g. '**/Migrations/**' or '**/*.g.dart'",
	"invalid path filter glob",
)

// glob is a validated gitignore-style pattern matched against a full entity
// path. Values are produced only by Compile, so match may skip re-validation.
type glob string

// match reports whether the full entity path satisfies the glob.
func (g glob) match(entity string) bool {
	return doublestar.MatchUnvalidated(string(g), entity)
}

// Spec holds the compiled include and exclude globs. The zero value matches
// nothing and is a no-op passthrough (see IsZero).
type Spec struct {
	Includes []glob
	Excludes []glob
}

// IsZero reports whether no include or exclude globs are set, so callers can
// skip the stage entirely.
func (s Spec) IsZero() bool {
	return len(s.Includes) == 0 && len(s.Excludes) == 0
}

// Compile validates and compiles the raw include and exclude glob strings. A
// malformed, empty, or oversized glob yields ErrInvalidGlob (exit 2). Empty
// input on both sides yields the zero Spec.
func Compile(includes, excludes []string) (Spec, error) {
	inc, err := compileGlobs(includes)
	if err != nil {
		return Spec{}, err
	}
	exc, err := compileGlobs(excludes)
	if err != nil {
		return Spec{}, err
	}
	return Spec{Includes: inc, Excludes: exc}, nil
}

// compileGlobs validates each pattern and returns the compiled globs in order,
// or nil for an empty input.
func compileGlobs(patterns []string) ([]glob, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	out := make([]glob, 0, len(patterns))
	for _, p := range patterns {
		if p == "" {
			return nil, ErrInvalidGlob.WithDetails(map[string]string{"glob": p})
		}
		if len(p) > maxPatternLen {
			return nil, ErrInvalidGlob.WithDetails(map[string]any{
				"glob_length": len(p), "max": maxPatternLen,
			})
		}
		if !doublestar.ValidatePattern(p) {
			return nil, ErrInvalidGlob.WithDetails(map[string]string{"glob": p})
		}
		out = append(out, glob(p))
	}
	return out, nil
}

// Apply keeps modifications whose Entity satisfies the include set (if any) and
// no exclude. A zero Spec is a no-op passthrough. The input slice is not
// mutated; a fresh slice is returned when filtering runs.
func Apply(mods []model.Modification, spec Spec) []model.Modification {
	if spec.IsZero() {
		return mods
	}
	out := make([]model.Modification, 0, len(mods))
	for _, m := range mods {
		if spec.keep(m.Entity) {
			out = append(out, m)
		}
	}
	return out
}

// keep applies exclude-after-include precedence to a single entity path.
func (s Spec) keep(entity string) bool {
	if len(s.Includes) > 0 && !anyMatch(s.Includes, entity) {
		return false
	}
	return !anyMatch(s.Excludes, entity)
}

// anyMatch reports whether entity matches any glob in globs.
func anyMatch(globs []glob, entity string) bool {
	for _, g := range globs {
		if g.match(entity) {
			return true
		}
	}
	return false
}
