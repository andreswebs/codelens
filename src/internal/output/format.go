package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/andreswebs/codelens/internal/terr"
)

// errUnknownFormat marks an unrecognized --format value. It is a usage error
// (exit 2): the value comes straight from a flag, so an unknown format is never
// an internal fault.
var errUnknownFormat = terr.New(
	"usage_error", 2,
	"choose one of: json, ndjson, csv, table",
	"unknown output format",
)

// Emit writes res to w in the named format, generically over any analysis's
// rows. columns are the ordered snake_case json keys of the row schema and
// drive the csv/table column set and order; they are the row schema's Names,
// passed as strings so the output package stays free of an analysis import.
//
// fields is honoured only by the json format (field projection); ndjson, csv,
// and table ignore it. An unknown format is a usage error.
func Emit(w io.Writer, format string, res Result, columns []string, fields string) error {
	switch format {
	case "", "json":
		return EmitProjected(w, res, fields)
	case "ndjson":
		return emitNDJSON(w, res)
	case "csv":
		return emitCSV(w, res, columns)
	case "table":
		return emitTable(w, res, columns)
	default:
		return errUnknownFormat.WithDetails(map[string]string{"format": format})
	}
}

// emitNDJSON writes one row object per line with no envelope wrapper, applied
// uniformly to every analysis (including scalar-shaped ones). Each line is the
// row's own JSON marshaling, so field order and key casing match the json
// format's rows. A result with no rows emits nothing.
func emitNDJSON(w io.Writer, res Result) error {
	rows, err := rowObjects(res.Rows)
	if err != nil {
		return err
	}
	for _, raw := range rows {
		if _, err := w.Write(raw); err != nil {
			return err
		}
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// emitCSV writes a kebab-case header derived from columns followed by one
// record per row, columns in schema order. It restores code-maat's CSV header
// casing for interop; rows are already sorted by the analysis, so log/sort
// order is preserved. --fields is intentionally ignored (json only).
func emitCSV(w io.Writer, res Result, columns []string) error {
	cw := csv.NewWriter(w)

	header := make([]string, len(columns))
	for i, c := range columns {
		header[i] = snakeToKebab(c)
	}
	if err := cw.Write(header); err != nil {
		return err
	}

	rows, err := rowMaps(res.Rows)
	if err != nil {
		return err
	}
	for _, row := range rows {
		rec := make([]string, len(columns))
		for i, c := range columns {
			rec[i] = cellString(row[c])
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

// emitTable writes a human-readable, tab-aligned table: a header of the
// snake_case column names followed by one padded row per record, columns in
// schema order. It is opt-in (never the default) for terminal use; --fields is
// ignored (json only).
func emitTable(w io.Writer, res Result, columns []string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	if _, err := fmt.Fprintln(tw, strings.Join(columns, "\t")); err != nil {
		return err
	}

	rows, err := rowMaps(res.Rows)
	if err != nil {
		return err
	}
	for _, row := range rows {
		cells := make([]string, len(columns))
		for i, c := range columns {
			cells[i] = cellString(row[c])
		}
		if _, err := fmt.Fprintln(tw, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// rowMaps marshals res.Rows and decodes each row into a key/value map, decoding
// numbers as json.Number so integer and centi-float values round-trip to their
// exact source text (no float widening or scientific notation) in csv/table
// cells.
func rowMaps(rows any) ([]map[string]any, error) {
	objs, err := rowObjects(rows)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(objs))
	for _, raw := range objs {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		m := map[string]any{}
		if err := dec.Decode(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// cellString renders one row value as a flat cell for csv/table: strings pass
// through, json.Number keeps its exact source text, bools become true/false, a
// missing (nil) value is empty, and anything else falls back to fmt.Sprint.
func cellString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case json.Number:
		return x.String()
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

// snakeToKebab converts a snake_case json key to the kebab-case header
// code-maat's CSV uses (n_authors -> n-authors).
func snakeToKebab(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

// rowObjects marshals res.Rows and splits it back into one raw JSON object per
// row, preserving each row's marshaled field order. A nil or empty rows value
// yields no objects rather than an error, so an empty-but-valid result formats
// cleanly.
func rowObjects(rows any) ([]json.RawMessage, error) {
	b, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 || string(b) == "null" {
		return nil, nil
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(b, &raws); err != nil {
		return nil, err
	}
	return raws, nil
}
