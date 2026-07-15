package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/andreswebs/codelens/internal/terr"
)

// ErrInvalidField is returned when --fields names a path that does not exist in
// the envelope. It is a usage error (exit 2); callers wrap it with the offending
// path and the set of valid paths.
var ErrInvalidField = terr.New("invalid_field", 2, "see `codelens schema --command CMD` for valid paths", "unknown field path")

// wildcard is the path segment that matches every key of a map-typed field.
const wildcard = "*"

// ValidateFields parses a comma-separated field-projection spec and validates
// each dotted path against the shape of envelope. An empty spec yields (nil,
// nil), meaning "no projection". An unknown path yields ErrInvalidField wrapped
// with the offending path and the sorted set of valid paths, so an agent can
// correct the request from the error alone.
func ValidateFields(paths string, envelope any) ([]string, error) {
	if strings.TrimSpace(paths) == "" {
		return nil, nil
	}

	valid := collectValidPaths(envelope)
	fields := splitFields(paths)
	for _, field := range fields {
		if !pathMatches(field, valid) {
			return nil, ErrInvalidField.
				WithDetails(map[string]any{"field": field, "valid": sortedKeys(valid)}).
				Wrap(fmt.Errorf("%q (valid: %s)", field, strings.Join(sortedKeys(valid), ", ")))
		}
	}
	return fields, nil
}

// ProjectFields re-marshals already-encoded envelope JSON down to only the
// requested field paths, always retaining schema_version and ok so the result
// stays a recognizable envelope. It operates on the decoded JSON tree rather
// than Go types, so projection is independent of the concrete row type.
func ProjectFields(data []byte, fields []string) ([]byte, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	tree := buildProjectionTree(fields)
	tree["schema_version"] = nil
	tree["ok"] = nil

	projected := applyProjection(root, tree)
	return json.Marshal(projected)
}

// EmitProjected writes envelope to w as JSON, projected to fieldsStr when it is
// non-empty. An empty fieldsStr is byte-identical to EmitJSON, so the projection
// path never perturbs the default output.
func EmitProjected(w io.Writer, envelope any, fieldsStr string) error {
	fields, err := ValidateFields(fieldsStr, envelope)
	if err != nil {
		return err
	}
	if fields == nil {
		return EmitJSON(w, envelope)
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	projected, err := ProjectFields(data, fields)
	if err != nil {
		return err
	}
	if _, err := w.Write(projected); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

// splitFields splits a comma-separated spec into trimmed, non-empty paths.
func splitFields(paths string) []string {
	parts := strings.Split(paths, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// pathMatches reports whether a requested dotted path is valid, treating a "*"
// segment in a valid path as matching any single requested segment (map keys).
func pathMatches(field string, valid map[string]struct{}) bool {
	if _, ok := valid[field]; ok {
		return true
	}
	reqSegs := strings.Split(field, ".")
	for v := range valid {
		if segmentsMatch(reqSegs, strings.Split(v, ".")) {
			return true
		}
	}
	return false
}

// segmentsMatch reports whether request and valid segment lists are equal,
// with a valid "*" segment matching any request segment at that position.
func segmentsMatch(req, valid []string) bool {
	if len(req) != len(valid) {
		return false
	}
	for i := range req {
		if valid[i] != wildcard && valid[i] != req[i] {
			return false
		}
	}
	return true
}

// collectValidPaths reflects over envelope and returns the set of every dotted
// path it exposes. Map-typed fields contribute a "*" wildcard path plus any
// keys present in the value.
func collectValidPaths(envelope any) map[string]struct{} {
	out := make(map[string]struct{})
	collectPaths(reflect.ValueOf(envelope), "", out)
	return out
}

// collectPaths walks v, recording each reachable json path under prefix. It
// descends through pointers and interfaces, into struct fields by json tag,
// into slice/array elements (using a zero element when empty), and across map
// keys plus the wildcard.
func collectPaths(v reflect.Value, prefix string, out map[string]struct{}) {
	v = deref(v)
	if !v.IsValid() {
		return
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			name, ok := jsonFieldName(f)
			if !ok {
				continue
			}
			path := join(prefix, name)
			out[path] = struct{}{}
			collectPaths(v.Field(i), path, out)
		}
	case reflect.Slice, reflect.Array:
		collectPaths(elemValue(v), prefix, out)
	case reflect.Map:
		out[join(prefix, wildcard)] = struct{}{}
		for _, k := range v.MapKeys() {
			key := fmt.Sprint(k.Interface())
			path := join(prefix, key)
			out[path] = struct{}{}
			collectPaths(v.MapIndex(k), path, out)
		}
	}
}

// deref unwraps pointers and interfaces, returning an invalid Value if a nil
// link is reached (nothing further to reflect on).
func deref(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

// elemValue returns the first element of a non-empty slice/array, or a zero
// value of the element type so an empty slice still yields its nested paths.
func elemValue(v reflect.Value) reflect.Value {
	if v.Len() > 0 {
		return v.Index(0)
	}
	return reflect.New(v.Type().Elem()).Elem()
}

// jsonFieldName returns the json object key for a struct field and whether it
// is serialized at all (false for json:"-"). An absent tag falls back to the
// field name, matching encoding/json.
func jsonFieldName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	name := tag
	if comma := strings.Index(tag, ","); comma >= 0 {
		name = tag[:comma]
	}
	if name == "" {
		name = f.Name
	}
	return name, true
}

// join concatenates a path prefix and a segment with a dot, or returns the
// segment alone at the top level.
func join(prefix, seg string) string {
	if prefix == "" {
		return seg
	}
	return prefix + "." + seg
}

// sortedKeys returns the map's keys in sorted order for deterministic messages.
func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// buildProjectionTree turns dotted field paths into a nested map. A nil value
// marks a leaf ("keep this subtree whole"); a nested map marks a branch to
// descend. A leaf already present wins over a deeper path, so the broader
// selection is honored.
func buildProjectionTree(fields []string) map[string]any {
	tree := map[string]any{}
	for _, field := range fields {
		segs := strings.Split(field, ".")
		cur := tree
		for i, seg := range segs {
			if i == len(segs)-1 {
				if _, exists := cur[seg]; !exists {
					cur[seg] = nil
				}
				break
			}
			next, ok := cur[seg].(map[string]any)
			if !ok {
				if _, isLeaf := cur[seg]; isLeaf {
					break
				}
				next = map[string]any{}
				cur[seg] = next
			}
			cur = next
		}
	}
	return tree
}

// applyProjection returns a copy of value keeping only what tree selects.
// Objects keep the named keys (and every key when tree holds "*"); arrays apply
// the same tree to each element; scalars pass through unchanged.
func applyProjection(value any, tree map[string]any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		if wildSub, ok := tree[wildcard]; ok {
			for key, child := range v {
				out[key] = projectChild(child, wildSub)
			}
		}
		for key, sub := range tree {
			if key == wildcard {
				continue
			}
			child, ok := v[key]
			if !ok {
				continue
			}
			out[key] = projectChild(child, sub)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, el := range v {
			out[i] = applyProjection(el, tree)
		}
		return out
	default:
		return value
	}
}

// projectChild descends one level: a nil subtree keeps the child whole, a map
// subtree recurses.
func projectChild(child, sub any) any {
	if sub == nil {
		return child
	}
	return applyProjection(child, sub.(map[string]any))
}
