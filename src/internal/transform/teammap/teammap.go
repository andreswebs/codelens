// Package teammap implements the team-mapping transform: it parses an
// author->team map (code-maat's `author,team` CSV form or a JSON object/array)
// and substitutes each Modification's author with their team, so social metrics
// aggregate at the team level.
//
// It is an optional pipeline stage, run after grouping and temporal collapsing
// when --team-map is supplied. Authors absent from the map are kept as-is (each
// becomes its own team), so mapping omissions surface quickly rather than being
// silently dropped.
package teammap

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"strings"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

// ErrInvalidTeamMap marks a malformed --team-map definition: a CSV row without
// exactly two columns, malformed JSON, or an unknown format. It is an input
// error (exit 3), consistent with the other mapping inputs the design classifies
// as input rather than usage errors. Callers wrap it with the offending detail
// via Wrap/WithDetails.
var ErrInvalidTeamMap = terr.New(
	"invalid_team_map", 3,
	"CSV rows are `author,team`; JSON is an object or array of {author,team}",
	"invalid team map",
)

// Parse reads an author->team map from r in the given format ("csv" (default) or
// "json"; the empty string means "csv") and returns it as an author->team
// lookup. Any malformed input yields ErrInvalidTeamMap (exit 3).
func Parse(r io.Reader, format string) (map[string]string, error) {
	switch format {
	case "", "csv":
		return parseCSV(r)
	case "json":
		return parseJSON(r)
	default:
		return nil, ErrInvalidTeamMap.WithDetails(map[string]string{"format": format})
	}
}

// parseCSV reads `author,team` rows. An optional leading `author,team` header
// row (case-insensitive, whitespace-trimmed) is skipped. Every data row must
// have exactly two columns; anything else is ErrInvalidTeamMap.
func parseCSV(r io.Reader) (map[string]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 2
	cr.TrimLeadingSpace = true

	teams := make(map[string]string)
	for row := 1; ; row++ {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrInvalidTeamMap.Wrap(err).WithDetails(map[string]any{"row": row})
		}
		author, team := strings.TrimSpace(rec[0]), strings.TrimSpace(rec[1])
		if row == 1 && strings.EqualFold(author, "author") && strings.EqualFold(team, "team") {
			continue
		}
		teams[author] = team
	}
	return teams, nil
}

// jsonEntry is the wire shape of a JSON-array team-map entry, mirroring the
// group transform's array form for symmetry.
type jsonEntry struct {
	Author string `json:"author"`
	Team   string `json:"team"`
}

// parseJSON accepts either the object form ({"author":"team",...}) or the array
// form ([{"author":..,"team":..},...]), dispatching on the first non-space byte.
func parseJSON(r io.Reader) (map[string]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, ErrInvalidTeamMap.Wrap(err)
	}
	switch first := firstNonSpace(data); first {
	case '{':
		var obj map[string]string
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, ErrInvalidTeamMap.Wrap(err)
		}
		return obj, nil
	case '[':
		var arr []jsonEntry
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, ErrInvalidTeamMap.Wrap(err)
		}
		teams := make(map[string]string, len(arr))
		for _, e := range arr {
			teams[e.Author] = e.Team
		}
		return teams, nil
	default:
		return nil, ErrInvalidTeamMap.WithDetails(map[string]string{"expected": "JSON object or array"})
	}
}

// firstNonSpace returns the first non-whitespace byte of data, or 0 if data is
// empty or all whitespace.
func firstNonSpace(data []byte) byte {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return 0
	}
	return trimmed[0]
}

// Apply substitutes each modification's author with its mapped team, leaving
// unmapped authors unchanged. The input slice is not mutated; a new slice of the
// same length and order is returned.
func Apply(mods []model.Modification, teams map[string]string) []model.Modification {
	out := make([]model.Modification, len(mods))
	copy(out, mods)
	for i := range out {
		if team, ok := teams[out[i].Author]; ok {
			out[i].Author = team
		}
	}
	return out
}
