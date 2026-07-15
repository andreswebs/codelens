package analysis

import (
	"github.com/andreswebs/codelens/internal/model"
)

// parseRow is one output row of the parse analysis: a single parsed
// modification record rendered verbatim. LocAdded and LocDeleted are pointers
// so a record without numstat (HasLoc false) omits the loc keys entirely rather
// than reporting a misleading zero; Binary is omitted unless set.
type parseRow struct {
	Entity     string `json:"entity"`
	Rev        string `json:"rev"`
	Date       string `json:"date"`
	Author     string `json:"author"`
	Message    string `json:"message"`
	LocAdded   *int   `json:"loc_added,omitempty"`
	LocDeleted *int   `json:"loc_deleted,omitempty"`
	Binary     bool   `json:"binary,omitempty"`
}

func init() {
	Register(parseDescriptor())
}

// parseDescriptor is the registered contract for the parse analysis (terse
// original alias "identity"). It is a function (rather than a package var) so
// tests can inspect the descriptor without depending on process-global
// registration state.
func parseDescriptor() Descriptor {
	return Descriptor{
		Name:    "parse",
		Aliases: []string{"identity"},
		Summary: "Dump parsed modification records (debug/interop)",
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "rev", Type: "string", Desc: "commit short hash"},
			{Name: "date", Type: "string", Desc: "commit date (YYYY-MM-dd)"},
			{Name: "author", Type: "string", Desc: "commit author (may be a mapped team)"},
			{Name: "message", Type: "string", Desc: "commit subject; \"-\" when absent"},
			{Name: "loc_added", Type: "int", Desc: "lines added; omitted when numstat is absent"},
			{Name: "loc_deleted", Type: "int", Desc: "lines deleted; omitted when numstat is absent"},
			{Name: "binary", Type: "bool", Desc: "whether git recorded binary numstat; omitted when false"},
		},
		ErrorCodes: []string{"empty_log"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runParse,
	}
}

// runParse emits the parsed modification records verbatim, in log order, after
// the pipeline's group/temporal/team-map transforms and before any analysis. It
// is a passthrough dump (a debug/interop escape hatch), so it applies no
// filtering, aggregation, or sorting. Loc metrics are carried only when the
// source record has them (HasLoc); the empty-log case is handled upstream by
// the parser.
func runParse(mods []model.Modification, _ Opts) (any, error) {
	rows := make([]parseRow, 0, len(mods))
	for _, m := range mods {
		row := parseRow{
			Entity:  m.Entity,
			Rev:     m.Rev,
			Date:    m.Date,
			Author:  m.Author,
			Message: m.Message,
			Binary:  m.Binary,
		}
		if m.HasLoc {
			added, deleted := m.LocAdded, m.LocDeleted
			row.LocAdded, row.LocDeleted = &added, &deleted
		}
		rows = append(rows, row)
	}

	return rows, nil
}
