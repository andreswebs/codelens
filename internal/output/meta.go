package output

// Meta is the descriptive half of a result envelope: everything the output layer
// needs to describe a payload but cannot derive from it. The command layer builds
// it from the analysis descriptor, which is why it is a plain struct of values
// rather than a descriptor reference: internal/output must not import
// internal/analysis (the dependency runs the other way).
type Meta struct {
	Analysis string
	Shape    string
	// Semantics maps each emitted payload field to its semantic type. The command
	// layer builds it from the descriptor (filtered per invocation flags and
	// adjusted for the active transforms), since only it knows which transforms ran.
	Semantics map[string]string
	// Transforms records the active pipeline transforms, nil when the pipeline was a
	// pass-through.
	Transforms map[string]any
	// Columns are the ordered snake_case payload field names the command declares.
	// They seed the --fields valid-path set so a projection stays valid on an
	// empty payload, where the data alone would expose no field paths.
	Columns []string
}

// NewResult wraps a payload in a success envelope, setting the invariants every
// result shares: the current schema version, ok=true, the analysis identity and
// shape from meta, and the payload count derived from payload. payload should be
// a slice for a table shape; a nil or non-slice value yields a zero RowCount.
// Params and the truncation metadata are populated by the caller after
// construction.
func NewResult(meta Meta, payload any) Result {
	return Result{
		SchemaVersion: SchemaVersion,
		OK:            true,
		Analysis:      meta.Analysis,
		Shape:         meta.Shape,
		Semantics:     meta.Semantics,
		Transforms:    meta.Transforms,
		RowCount:      RowLen(payload),
		Payload:       payload,
	}
}
