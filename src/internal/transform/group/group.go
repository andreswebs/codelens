// Package group implements the architectural-grouping transform: it parses a
// layer-mapping specification (code-maat's `pattern => name` text form or a JSON
// array) and remaps each Modification's entity to the first matching group,
// dropping entities that match no group.
//
// It is an optional pipeline stage, run after parsing and before analysis when
// --group is supplied, so downstream analyses aggregate at the layer level.
package group

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

// maxPatternLen bounds a group pattern's length. Go's regexp is RE2 (linear
// time, no catastrophic backtracking), so a length cap is the only guard needed
// against a pathological definition.
const maxPatternLen = 1000

// separator divides a text-form line into its pattern and group name.
const separator = "=>"

// ErrInvalidGroup marks a malformed --group definition: a line without the
// `=>` separator, an empty pattern or name, an oversized or uncompilable
// pattern, malformed JSON, or an unknown format. It is a usage error (exit 2)
// because the definition is supplied directly via flags, never derived from the
// analyzed log. Callers wrap it with the offending detail via Wrap/WithDetails.
var ErrInvalidGroup = terr.New(
	"invalid_group", 2,
	"text lines are `pattern => name`; see `codelens schema --command CMD`",
	"invalid group definition",
)

// Spec is a single compiled grouping rule: an anchored pattern and the group
// name that matching entities are remapped to.
type Spec struct {
	// Pattern is the compiled, anchored matcher for entity paths.
	Pattern *regexp.Regexp
	// Name is the group every entity matching Pattern is remapped to.
	Name string
}

// Parse reads a layer-mapping specification from r in the given format ("text"
// or "json"; the empty string means "text") and returns the compiled specs in
// definition order. First-match ordering at Apply time follows this order, so
// callers must keep more specific rules first. Any malformed input yields
// ErrInvalidGroup (exit 2).
func Parse(r io.Reader, format string) ([]Spec, error) {
	switch format {
	case "", "text":
		return parseText(r)
	case "json":
		return parseJSON(r)
	default:
		return nil, ErrInvalidGroup.WithDetails(map[string]string{"format": format})
	}
}

// parseText parses the `pattern => name` line form, ignoring blank lines and
// trimming surrounding whitespace around both the pattern and the name.
func parseText(r io.Reader) ([]Spec, error) {
	var specs []Spec
	sc := bufio.NewScanner(r)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		pattern, name, ok := strings.Cut(text, separator)
		if !ok {
			return nil, ErrInvalidGroup.WithDetails(map[string]any{
				"line": line, "content": text,
			})
		}
		spec, err := makeSpec(strings.TrimSpace(pattern), strings.TrimSpace(name))
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	if err := sc.Err(); err != nil {
		return nil, ErrInvalidGroup.Wrap(err)
	}
	return specs, nil
}

// jsonSpec is the wire shape of a JSON-form grouping rule.
type jsonSpec struct {
	Pattern string `json:"pattern"`
	Name    string `json:"name"`
}

// parseJSON parses a JSON array of {"pattern","name"} objects, applying the same
// anchoring and validation as the text form.
func parseJSON(r io.Reader) ([]Spec, error) {
	var raw []jsonSpec
	dec := json.NewDecoder(r)
	if err := dec.Decode(&raw); err != nil {
		return nil, ErrInvalidGroup.Wrap(err)
	}
	specs := make([]Spec, 0, len(raw))
	for _, js := range raw {
		spec, err := makeSpec(js.Pattern, js.Name)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// makeSpec validates a pattern/name pair and compiles the anchored matcher. A
// pattern starting with "^" is compiled verbatim; otherwise it is anchored as
// "^<pattern>/" for a path-prefix match, matching code-maat's grouper.
func makeSpec(pattern, name string) (Spec, error) {
	if pattern == "" || name == "" {
		return Spec{}, ErrInvalidGroup.WithDetails(map[string]any{
			"pattern": pattern, "name": name,
		})
	}
	if len(pattern) > maxPatternLen {
		return Spec{}, ErrInvalidGroup.WithDetails(map[string]any{
			"pattern_length": len(pattern), "max": maxPatternLen,
		})
	}
	expr := pattern
	if !strings.HasPrefix(pattern, "^") {
		expr = "^" + pattern + "/"
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return Spec{}, ErrInvalidGroup.Wrap(err).WithDetails(map[string]any{
			"pattern": expr,
		})
	}
	return Spec{Pattern: re, Name: name}, nil
}

// Apply remaps each modification's entity to the name of the first spec whose
// pattern matches it, preserving input order. Modifications matching no spec are
// dropped. The input slice is not mutated; a new slice is returned.
func Apply(mods []model.Modification, specs []Spec) []model.Modification {
	out := make([]model.Modification, 0, len(mods))
	for _, m := range mods {
		for _, s := range specs {
			if s.Pattern.MatchString(m.Entity) {
				m.Entity = s.Name
				out = append(out, m)
				break
			}
		}
	}
	return out
}
