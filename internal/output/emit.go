package output

import (
	"encoding/json"
	"io"
)

// EmitJSON marshals v as compact JSON to w, followed by a single newline. It is
// the one place stdout results are serialized, so every command emits the same
// byte shape.
func EmitJSON(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}
