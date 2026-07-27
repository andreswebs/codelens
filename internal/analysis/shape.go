package analysis

// Shape names the topology of a command's output payload, and the payload key
// follows from it: "table" carries rows, "graph" carries nodes and edges, and so
// on. It is declared per command (not per invocation): one command has exactly
// one shape, and alternate views of the same data are downstream derivations, not
// output modes.
const (
	ShapeTable  = "table"
	ShapeTree   = "tree"
	ShapeGraph  = "graph"
	ShapeMatrix = "matrix"
	ShapeSeries = "series"
	// ShapeText is the escape hatch for a helper whose stdout is a bare string
	// meant to be copied and run (print-log-command), not a data payload. It is
	// declared so `schema --command` tells an agent not to pipe that stdout into a
	// JSON parser.
	ShapeText = "text"
)

// Shapes is the closed set of shape names, in declaration order. A conformance
// test pins every descriptor's Shape to a member.
func Shapes() []string {
	return []string{ShapeTable, ShapeTree, ShapeGraph, ShapeMatrix, ShapeSeries, ShapeText}
}

// ValidShape reports whether s is a member of the closed set.
func ValidShape(s string) bool {
	for _, sh := range Shapes() {
		if sh == s {
			return true
		}
	}
	return false
}
