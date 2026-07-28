package analysis

// An AggRole names how a column's values behave under aggregation: whether
// summing them across rows is meaningful (additive), meaningless (intensive),
// or inapplicable because the column is a category or an identity. It is the
// third enum of the package's vocabulary, alongside Semantic and Shape.
//
// AggRole is the one enum that takes a member of another enum as input (a role
// is derived from a Semantic), so it is the place two string vocabularies meet
// in one expression; the named types make mixing them a compile error rather
// than a silently-false comparison.
//
// The set is four members, not the five of Flint's aggRole it is borrowed
// from: signed-additive is omitted because codelens has no signed measure
// (added and deleted are separate non-negative columns, never a net delta).
// Declaring an unreachable member repeats the mistake ADR 0008 corrected for
// the shape enum, whose rule is that a member is added by the change that
// makes it reachable. A net-churn column would bring signed-additive with it.
type AggRole string

// The closed aggregation-role vocabulary, in declaration order (see AggRoles).
const (
	// AggAdditive marks a measure whose parts sum to a meaningful total.
	AggAdditive AggRole = "additive"
	// AggIntensive marks a level or proportion; a sum of intensive values is
	// not a value (order statistics like median and max are the legal
	// aggregates). Note that duration_months is intensive HERE even though
	// Flint registers Duration as additive: Flint is right for its domain
	// (time spent is a quantity) and wrong for ours, because codelens's only
	// duration column is code-age's age_months, "months since the entity last
	// changed", which is a level. Summing the ages of 500 files is
	// meaningless; the useful statistics are median and max. Do not align
	// this assignment with Flint.
	AggIntensive AggRole = "intensive"
	// AggDimension marks a grouping key, not a measure.
	AggDimension AggRole = "dimension"
	// AggIdentifier marks a value that is neither aggregated nor grouped on.
	// It does NOT imply uniqueness: commit_id is unique and text (a commit
	// subject) is not, and both are equally unaggregatable. Grouping by free
	// prose yields one group per row. The role is defined by BEHAVIOUR, not
	// by cardinality.
	AggIdentifier AggRole = "identifier"
)

// AggRoles returns the closed set, in declaration order.
func AggRoles() []AggRole {
	return []AggRole{
		AggAdditive,
		AggIntensive,
		AggDimension,
		AggIdentifier,
	}
}

// ValidAggRole reports whether r is a member of the closed set.
func ValidAggRole(r AggRole) bool {
	for _, role := range AggRoles() {
		if role == r {
			return true
		}
	}
	return false
}

// AggregationRoles returns the full semantic-to-role catalog: one entry per
// member of the semantic vocabulary. It is what the schema command list
// publishes, and publishing it is the first place the closed semantic set
// appears on the wire at all (per-command schemas list semantics only
// per-column). Built from Semantics() rather than written out, so it cannot
// disagree with the vocabulary it publishes.
func AggregationRoles() map[Semantic]AggRole {
	m := make(map[Semantic]AggRole, len(Semantics()))
	for _, s := range Semantics() {
		m[s] = AggRoleOf(s)
	}
	return m
}

// AggRoleOf returns the aggregation role of a semantic, or "" if s is not a
// member of the semantic vocabulary. It returns the zero value rather than
// panicking because it is reachable with arbitrary input once a caller reads a
// semantic from data (unlike payloadKey, whose panic is reachable only through
// a descriptor bug). The switch deliberately has no default branch: an
// unhandled semantic falls through to "" and the exhaustiveness test fires.
func AggRoleOf(s Semantic) AggRole {
	switch s {
	case SemanticCount, SemanticLoc:
		return AggAdditive
	case SemanticPercentage, SemanticRatio, SemanticDurationMonths:
		return AggIntensive
	case SemanticFilepath, SemanticPerson, SemanticDate, SemanticLabel, SemanticFlag:
		return AggDimension
	case SemanticCommitID, SemanticText:
		return AggIdentifier
	}
	return ""
}
