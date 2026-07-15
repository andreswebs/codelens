package output

import "reflect"

// NewResult wraps an analysis's rows in a success envelope, setting the
// invariants every result shares: the current schema version, ok=true, the
// analysis name, and the row count derived from rows. rows must be a slice; a
// nil or non-slice value yields a zero RowCount. Params and the truncation
// metadata are populated by the caller after construction.
func NewResult(analysis string, rows any) Result {
	return Result{
		SchemaVersion: SchemaVersion,
		OK:            true,
		Analysis:      analysis,
		RowCount:      RowLen(rows),
		Rows:          rows,
	}
}

// RowLen reports the number of rows in a result payload. It is the single
// reflection site for row counting, shared by NewResult and by the --rows
// truncation in the command layer. A nil or non-slice value has length 0.
func RowLen(rows any) int {
	if rows == nil {
		return 0
	}
	rv := reflect.ValueOf(rows)
	if rv.Kind() != reflect.Slice {
		return 0
	}
	return rv.Len()
}
