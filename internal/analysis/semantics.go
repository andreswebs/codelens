package analysis

// A semantic names what a payload field MEANS, as distinct from its JSON type: a
// filepath rather than a string, a percentage rather than an int. It is the asset
// only codelens can provide, because it authored the data, and it is what lets a
// downstream renderer derive a chart without domain knowledge.
//
// A semantic is a bare enum string; range and unit are implied by the name and
// fixed here, so the map projects to a chart-spec input (Flint's semantic_types)
// unchanged:
//   - Percentage is an integer 0-100; Ratio is a float 0-1. A field is one or the
//     other, never both.
//   - Count is a tally of things; Loc is a count of LINES. The split is not
//     cosmetic: lines are the conventional size channel of a treemap while
//     frequencies are the colour channel, and a renderer cannot tell them apart
//     from the type alone.
const (
	SemanticFilepath       = "filepath"        // repository path, splittable on "/"
	SemanticPerson         = "person"          // actor name (an author, or a team under --team-map)
	SemanticDate           = "date"            // calendar date, YYYY-MM-dd
	SemanticCommitID       = "commit_id"       // opaque commit identifier
	SemanticText           = "text"            // free prose; never a plottable category
	SemanticLabel          = "label"           // categorical name
	SemanticFlag           = "flag"            // boolean
	SemanticCount          = "count"           // tally of things
	SemanticLoc            = "loc"             // line count (a size measure)
	SemanticPercentage     = "percentage"      // integer 0-100
	SemanticRatio          = "ratio"           // float 0-1
	SemanticDurationMonths = "duration_months" // whole calendar months
)

// Semantics returns the closed set, in declaration order. A conformance test pins
// every declared column's Semantic to a member, so a new analysis cannot invent
// one.
func Semantics() []string {
	return []string{
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
func ValidSemantic(s string) bool {
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
func SemanticsOf(d Descriptor, omit map[string]bool) map[string]string {
	m := make(map[string]string, len(d.RowSchema))
	for _, c := range d.RowSchema {
		if c.FlagGated != "" && omit[c.FlagGated] {
			continue
		}
		m[c.Name] = c.Semantic
	}
	return m
}
