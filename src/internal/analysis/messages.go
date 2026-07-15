package analysis

import (
	"regexp"
	"sort"

	"github.com/andreswebs/codelens/internal/analysis/calc"
	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

// maxExpressionLen bounds a --expression pattern's length. Go's regexp is RE2
// (linear-time matching), so an oversized or malformed pattern is a usage error
// rather than a matching hazard; the length cap keeps a pathological pattern
// from ever reaching the compiler.
const maxExpressionLen = 1000

// ErrInvalidExpression marks a missing, oversized, or uncompilable --expression.
// It is a usage error (exit code 2): the caller must supply a valid, bounded
// regular expression.
var ErrInvalidExpression = terr.New(
	"invalid_expression",
	2,
	"supply a valid, bounded regular expression via --expression",
	"invalid --expression value",
)

// ErrMissingMessages reports that the log carries no commit messages, so a
// messages analysis has nothing to match. The git2 log's extended format keeps
// the subject; a stock 3-field log defaults every message to "-" (exit code 3,
// input error).
var ErrMissingMessages = terr.New(
	"missing_messages",
	3,
	"generate the log with commit subjects (see `codelens print-log-command`)",
	"the VCS data has no commit messages",
)

// messagesRow is one output row of the messages analysis: an entity and how
// many of its revisions carry a commit message matching the expression.
type messagesRow struct {
	Entity  string `json:"entity"`
	Matches int    `json:"matches"`
}

func init() {
	Register(messagesDescriptor())
}

// messagesDescriptor is the registered contract for the messages analysis. It
// is a function (rather than a package var) so tests can inspect the descriptor
// without depending on process-global registration state.
func messagesDescriptor() Descriptor {
	return Descriptor{
		Name:    "messages",
		Summary: "Entity frequency for commit-message regex matches",
		Flags: []Flag{
			{Name: "expression", Type: "string", Default: "", Required: true, Desc: "regular expression matched against commit messages"},
		},
		RowSchema: []Column{
			{Name: "entity", Type: "string", Desc: "module path"},
			{Name: "matches", Type: "int", Desc: "revisions whose commit message matched the expression"},
		},
		ErrorCodes: []string{"empty_log", "missing_messages", "invalid_expression"},
		ExitCodes:  []int{0, 2, 3, 1},
		Run:        runMessages,
	}
}

// runMessages counts, per entity, the distinct revisions whose commit message
// matches --expression. It requires a valid, bounded expression (a missing,
// oversized, or uncompilable pattern is an invalid_expression usage error) and
// a log that carries messages (a message-only stock log is a missing_messages
// input error). Rows are ordered by [matches, entity] descending, matching the
// original's sort; entity is the group key so the ordering is fully
// deterministic for --rows truncation.
func runMessages(mods []model.Modification, opts Opts) (any, error) {
	re, err := compileExpression(opts.Expression)
	if err != nil {
		return nil, err
	}

	if err := requireMessages(mods); err != nil {
		return nil, err
	}

	matched := make([]model.Modification, 0, len(mods))
	for _, m := range mods {
		if re.MatchString(m.Message) {
			matched = append(matched, m)
		}
	}

	groups := calc.GroupBy(matched, func(m model.Modification) string { return m.Entity })

	rows := make([]messagesRow, 0, len(groups))
	for _, g := range groups {
		revs := make([]string, 0, len(g.Items))
		for _, m := range g.Items {
			revs = append(revs, m.Rev)
		}
		rows = append(rows, messagesRow{
			Entity:  g.Key,
			Matches: len(calc.Distinct(revs)),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Matches != rows[j].Matches {
			return rows[i].Matches > rows[j].Matches
		}
		return rows[i].Entity > rows[j].Entity
	})

	return rows, nil
}

// compileExpression validates and compiles the required --expression. An empty
// or oversized pattern, or one that fails to compile, is an invalid_expression
// usage error carrying the offending value in its details.
func compileExpression(expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, ErrInvalidExpression.WithDetails(map[string]any{"reason": "missing"})
	}
	if len(expr) > maxExpressionLen {
		return nil, ErrInvalidExpression.WithDetails(map[string]any{
			"expression_length": len(expr), "max": maxExpressionLen,
		})
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, ErrInvalidExpression.Wrap(err).WithDetails(map[string]any{"expression": expr})
	}
	return re, nil
}

// requireMessages returns ErrMissingMessages when mods contains modifications
// but none of them carry a real commit message (every subject is the "-"
// placeholder of a stock 3-field log). An empty slice is not a messages error:
// absence of data is handled upstream (empty_log), not here.
func requireMessages(mods []model.Modification) error {
	if len(mods) == 0 {
		return nil
	}
	for _, m := range mods {
		if m.Message != "-" {
			return nil
		}
	}
	return ErrMissingMessages
}
