// Package output builds and emits codelens's result and error envelopes. Every
// command renders through it: successful runs marshal a Result, failures render
// an error envelope and resolve a process exit code from the error's kind.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SchemaVersion is the version of the output envelope contract. It is bumped
// only on a breaking change to the envelope shape.
const SchemaVersion = 1

// Result is the success envelope wrapping one analysis's payload. Params echoes
// the effective tuning options so a result is self-documenting; TotalCount and
// Truncated are populated only when --rows caps the output, and are omitted
// otherwise so an uncapped result is unambiguous. The struct tags are all
// json:"-" deliberately: MarshalJSON drives the wire output, so a tag other than
// "-" would mislead a reader into thinking encoding/json's struct walk applies.
type Result struct {
	SchemaVersion int    `json:"-"`
	OK            bool   `json:"-"`
	Analysis      string `json:"-"`
	Shape         string `json:"-"`
	// Semantics maps each emitted payload field to its semantic type (see
	// analysis.Semantics). It is always present, even when empty, so a consumer can
	// read it unconditionally; the marshaler writes {} rather than null when it is
	// nil (ADR 0006's absent-map rule).
	Semantics map[string]string `json:"-"`
	// Transforms records which pipeline transforms ran, omitted entirely when the
	// pipeline was a pass-through. It is what justifies an adjusted semantic (a
	// grouped entity reported as a label rather than a filepath).
	Transforms map[string]any `json:"-"`
	Params     map[string]any `json:"-"`
	RowCount   int            `json:"-"`
	TotalCount int            `json:"-"`
	Truncated  bool           `json:"-"`
	// Payload is the shape's data, written under the key Shape dictates (see
	// payloadKey): rows for a table. Keeping one payload field rather than one per
	// shape makes a mismatched key (a table result emitting "nodes") unrepresentable.
	Payload any `json:"-"`
}

// MarshalJSON writes the envelope with a stable key order and the payload under
// the key its shape dictates. The order is metadata first, payload last, so
// `head` on a large result shows the descriptive fields. Key order is part of the
// golden contract (ADR 0007), so it is fixed here rather than left to struct
// field order; a map[string]any would not do, because encoding/json sorts map
// keys alphabetically and would scramble the contract order.
//
// The order is: schema_version, ok, analysis, shape, semantics, transforms,
// params, row_count, total_count, truncated, <payload_key>. semantics is always
// present (a nil map marshals as {}); transforms is omitted when the pipeline was
// a pass-through. Omission preserves the previous behaviour otherwise: params
// omitted when nil, total_count omitted when zero, truncated omitted when false;
// row_count is always present.
func (r Result) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	w := &fieldWriter{buf: &buf}
	if err := w.field("schema_version", r.SchemaVersion); err != nil {
		return nil, err
	}
	if err := w.field("ok", r.OK); err != nil {
		return nil, err
	}
	if err := w.field("analysis", r.Analysis); err != nil {
		return nil, err
	}
	if err := w.field("shape", r.Shape); err != nil {
		return nil, err
	}
	// semantics is always present; a nil map marshals as {} rather than null so a
	// consumer reads it unconditionally.
	semantics := r.Semantics
	if semantics == nil {
		semantics = map[string]string{}
	}
	if err := w.field("semantics", semantics); err != nil {
		return nil, err
	}
	// transforms is omitted entirely when the pipeline was a pass-through.
	if len(r.Transforms) > 0 {
		if err := w.field("transforms", r.Transforms); err != nil {
			return nil, err
		}
	}
	if r.Params != nil {
		if err := w.field("params", r.Params); err != nil {
			return nil, err
		}
	}
	if err := w.field("row_count", r.RowCount); err != nil {
		return nil, err
	}
	if r.TotalCount != 0 {
		if err := w.field("total_count", r.TotalCount); err != nil {
			return nil, err
		}
	}
	if r.Truncated {
		if err := w.field("truncated", r.Truncated); err != nil {
			return nil, err
		}
	}
	if err := w.field(payloadKey(r.Shape), r.Payload); err != nil {
		return nil, err
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// wildcardPaths returns the dotted paths under which a map-typed envelope field
// accepts an arbitrary key, so `--fields params.*` validates against a projection
// even though the concrete keys vary per run. Only open-keyed maps are listed;
// the payload and its fixed-key row elements are not, so `rows.*` stays invalid.
func (r Result) wildcardPaths() []string {
	var out []string
	if r.Params != nil {
		out = append(out, "params."+wildcard)
	}
	return out
}

// fieldWriter appends comma-separated "key":value pairs to a buffer, marshaling
// each value with encoding/json so nested types keep their own tag-driven shape.
type fieldWriter struct {
	buf   *bytes.Buffer
	wrote bool
}

func (w *fieldWriter) field(key string, val any) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	if w.wrote {
		w.buf.WriteByte(',')
	}
	w.wrote = true
	keyBytes, err := json.Marshal(key)
	if err != nil {
		return err
	}
	w.buf.Write(keyBytes)
	w.buf.WriteByte(':')
	w.buf.Write(b)
	return nil
}

// payloadKey returns the JSON key the shape's payload is written under. "table"
// is the only data shape, and it carries "rows"; "text" never reaches this
// function, because print-log-command writes a bare command line straight to the
// writer and never builds a Result. An unrecognized shape is a descriptor typo,
// reachable only via programmer error, and panics as the backstop, matching
// toCLIFlag's treatment of an unsupported flag type.
func payloadKey(shape string) string {
	switch shape {
	case "table":
		return "rows"
	}
	panic(fmt.Sprintf("output: unknown payload shape %q", shape))
}
