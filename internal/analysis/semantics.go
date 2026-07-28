package analysis

// A Semantic names what a payload field MEANS, as distinct from its JSON type: a
// filepath rather than a string, a percentage rather than an int. It is the asset
// only codelens can provide, because it authored the data, and it is what lets a
// downstream renderer derive a chart without domain knowledge.
//
// A semantic is a bare enum string (a named type marshals identically); range and
// unit are implied by the name and fixed here, so the map projects to a
// chart-spec input (Flint's semantic_types) unchanged:
//   - Percentage is an integer 0-100; Ratio is a float 0-1. A field is one or the
//     other, never both.
//   - Count is a tally of things; Loc is a count of LINES. The split is not
//     cosmetic: lines are the conventional size channel of a treemap while
//     frequencies are the colour channel, and a renderer cannot tell them apart
//     from the type alone.
type Semantic string

// The closed semantic vocabulary, in declaration order (see Semantics).
const (
	SemanticFilepath       Semantic = "filepath"        // repository path, splittable on "/"
	SemanticPerson         Semantic = "person"          // actor name (an author, or a team under --team-map)
	SemanticDate           Semantic = "date"            // calendar date, YYYY-MM-dd
	SemanticCommitID       Semantic = "commit_id"       // opaque commit identifier
	SemanticText           Semantic = "text"            // free prose; never a plottable category
	SemanticLabel          Semantic = "label"           // categorical name
	SemanticFlag           Semantic = "flag"            // boolean
	SemanticCount          Semantic = "count"           // tally of things
	SemanticLoc            Semantic = "loc"             // line count (a size measure)
	SemanticPercentage     Semantic = "percentage"      // integer 0-100
	SemanticRatio          Semantic = "ratio"           // float 0-1
	SemanticDurationMonths Semantic = "duration_months" // whole calendar months
)

// Semantics returns the closed set, in declaration order. A conformance test pins
// every declared column's Semantic to a member, so a new analysis cannot invent
// one.
func Semantics() []Semantic {
	return []Semantic{
		SemanticFilepath,
		SemanticPerson,
		SemanticDate,
		SemanticCommitID,
		SemanticText,
		SemanticLabel,
		SemanticFlag,
		SemanticCount,
		SemanticLoc,
		SemanticPercentage,
		SemanticRatio,
		SemanticDurationMonths,
	}
}

// ValidSemantic reports whether s is a member of the closed set.
func ValidSemantic(s Semantic) bool {
	for _, sem := range Semantics() {
		if sem == s {
			return true
		}
	}
	return false
}

// SemanticsOf projects a descriptor's declared columns to the field-to-semantic
// map an envelope carries, omitting any column excluded by the invocation.
// Semantics track FLAGS, never data: a column gated behind a flag that was not
// supplied is absent (its field will never appear in any row), while a column
// whose presence varies per row (parse's loc metrics) is always listed. That
// keeps the map deterministic for a given command line.
//
// omit names the flags a run did NOT supply; a column whose FlagGated flag is in
// omit is dropped. It is built by the command layer from the descriptor's gated
// columns, so the output layer needs no analysis-specific knowledge.
func SemanticsOf(d Descriptor, omit map[string]bool) map[string]Semantic {
	m := make(map[string]Semantic, len(d.RowSchema))
	for _, c := range d.RowSchema {
		if c.FlagGated != "" && omit[c.FlagGated] {
			continue
		}
		m[c.Name] = c.Semantic
	}
	return m
}
