package analysis

// A Shape names the topology of a command's output payload, and the payload key
// follows from it: "table" carries rows. It is declared per command (not per
// invocation): one command has exactly one shape, and alternate views of the same
// data are downstream derivations, not output modes.
//
// The set holds only the shapes codelens actually emits. A shape is added by the
// change that makes it emittable, never ahead of it: `schema` is the runtime
// contract an agent relies on, so a declared shape no command can produce would
// be an unkeepable promise. A hierarchy shape and a graph shape are both
// anticipated and will arrive with the analyses that need them (see ADR 0008).
type Shape string

// The closed shape set, in declaration order (see Shapes).
const (
	ShapeTable Shape = "table"
	// ShapeText is the escape hatch for a helper whose stdout is a bare string
	// meant to be copied and run (print-log-command), not a data payload. It is
	// declared so `schema --command` tells an agent not to pipe that stdout into a
	// JSON parser.
	ShapeText Shape = "text"
)

// Shapes is the closed set of shape names, in declaration order. A conformance
// test pins every descriptor's Shape to a member.
func Shapes() []Shape {
	return []Shape{ShapeTable, ShapeText}
}

// ValidShape reports whether s is a member of the closed set.
func ValidShape(s Shape) bool {
	for _, sh := range Shapes() {
		if sh == s {
			return true
		}
	}
	return false
}
