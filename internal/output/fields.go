package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/andreswebs/codelens/internal/terr"
)

// ErrInvalidField is returned when --fields names a path that does not exist in
// the envelope. It is a usage error (exit 64); callers wrap it with the offending
// path and the set of valid paths.
var ErrInvalidField = terr.New("invalid_field", 64, "see `codelens schema --command CMD` for valid paths", "unknown field path")

// wildcard is the path segment that matches every key of a map-typed field.
const wildcard = "*"

// ValidateFields parses a comma-separated field-projection spec and validates
// each dotted path against the envelope's ACTUAL emitted shape, unioned with the
// payload field paths the command declares. An empty spec yields (nil, nil),
// meaning "no projection". An unknown path yields ErrInvalidField wrapped with the
// offending path and the sorted set of valid paths, so an agent can correct the
// request from the error alone.
//
// Taking the declared columns from the schema rather than from the data is what
// keeps a projection valid on an empty payload: the data alone exposes no field
// paths when the payload is []. declared is the ordered column-name list for the
// command's payload; it is prefixed with the payload key the envelope's shape
// dictates.
func ValidateFields(paths string, envelope any, declared []string) ([]string, error) {
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	return validateFieldsData(paths, data, envelope, declared)
}

// validateFieldsData validates paths against already-marshaled envelope bytes so
// the emit path can reuse the marshal rather than encoding twice. envelope is
// still passed to recover the payload key and the map-typed wildcard paths, which
// the JSON bytes alone cannot distinguish (a fixed-key row object and an
// open-key params object both decode to a JSON object).
func validateFieldsData(paths string, data []byte, envelope any, declared []string) ([]string, error) {
	if strings.TrimSpace(paths) == "" {
		return nil, nil
	}

	valid, err := collectJSONPaths(data)
	if err != nil {
		return nil, err
	}
	if r, ok := envelope.(Result); ok {
		key := payloadKey(r.Shape)
		for _, c := range declared {
			valid[join(key, c)] = struct{}{}
		}
		for _, p := range r.wildcardPaths() {
			valid[p] = struct{}{}
		}
	}

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
// requested field paths, always retaining schema_version, ok, shape, transforms
// (when present), and a semantics map narrowed to the surviving payload fields, so
// the result stays a recognizable, self-describing envelope (D6). It operates on
// the decoded JSON tree rather than Go types, so projection is independent of the
// concrete payload type. payloadKey is the envelope's payload key (e.g. "rows"),
// used to intersect semantics with the fields the projection kept.
func ProjectFields(data []byte, fields []string, payloadKey string) ([]byte, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	tree := buildProjectionTree(fields)
	tree["schema_version"] = nil
	tree["ok"] = nil
	tree["shape"] = nil      // always self-describing, even when projected (D6)
	tree["transforms"] = nil // retained when present, it justifies an adjusted semantic (D6)

	projected := applyProjection(root, tree)
	if m, ok := projected.(map[string]any); ok {
		m["semantics"] = projectSemantics(root, tree, payloadKey)
	}
	return json.Marshal(projected)
}

// projectSemantics narrows the semantics map to the payload fields the projection
// kept, so a projected envelope stays self-describing without advertising fields
// it dropped. A projection that keeps the whole payload (a "rows" leaf or a
// wildcard under it) keeps every semantic; otherwise it intersects the semantics
// keys with the payload subtree's keys. A projection that keeps no payload field
// yields an empty map, not a missing key.
func projectSemantics(root any, tree map[string]any, payloadKey string) map[string]any {
	rootMap, ok := root.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	sem, ok := rootMap["semantics"].(map[string]any)
	if !ok {
		return map[string]any{}
	}

	sub, present := tree[payloadKey]
	if !present {
		return map[string]any{}
	}
	subTree, ok := sub.(map[string]any)
	if !ok {
		// A leaf (the whole payload was requested, e.g. --fields rows): keep all.
		return sem
	}
	if _, wild := subTree[wildcard]; wild {
		return sem
	}

	out := make(map[string]any, len(subTree))
	for field := range subTree {
		if v, ok := sem[field]; ok {
			out[field] = v
		}
	}
	return out
}

// EmitProjected writes envelope to w as JSON, projected to fieldsStr when it is
// non-empty. An empty fieldsStr is byte-identical to EmitJSON, so the projection
// path never perturbs the default output. declared is the command's payload
// column-name list, seeding the valid-path set so a projection stays valid on an
// empty payload.
func EmitProjected(w io.Writer, envelope any, fieldsStr string, declared []string) error {
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	fields, err := validateFieldsData(fieldsStr, data, envelope, declared)
	if err != nil {
		return err
	}
	if fields == nil {
		if _, err := w.Write(data); err != nil {
			return err
		}
		_, err = w.Write([]byte{'\n'})
		return err
	}

	key := ""
	if r, ok := envelope.(Result); ok {
		key = payloadKey(r.Shape)
	}
	projected, err := ProjectFields(data, fields, key)
	if err != nil {
		return err
	}
	if _, err := w.Write(projected); err != nil {
		return err
	}
	_, err = w.Write([]byte{'\n'})
	return err
}

// collectJSONPaths decodes marshaled envelope bytes and returns the set of every
// dotted path the JSON exposes. Objects contribute each key and recurse; arrays
// recurse into their first element, so a non-empty payload exposes its row field
// paths while an empty one exposes none (the declared columns cover that case).
func collectJSONPaths(data []byte) (map[string]struct{}, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	walkJSON(root, "", out)
	return out, nil
}

// walkJSON records each reachable dotted path of v under prefix.
func walkJSON(v any, prefix string, out map[string]struct{}) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			path := join(prefix, k)
			out[path] = struct{}{}
			walkJSON(child, path, out)
		}
	case []any:
		if len(t) > 0 {
			walkJSON(t[0], prefix, out)
		}
	}
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
