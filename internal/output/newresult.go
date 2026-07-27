package output

import "reflect"

// RowLen reports the number of rows in a result payload. It is the single
// reflection site for row counting, shared by NewResult and by the --rows
// truncation in the command layer. A nil or non-slice value has length 0, so a
// future non-slice payload (a tree or graph) is simply not counted as rows.
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
