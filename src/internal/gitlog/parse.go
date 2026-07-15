package gitlog

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/andreswebs/codelens/internal/model"
)

// hashPattern and datePattern validate the prelude's first two fields: a git
// short hash and a canonical YYYY-MM-dd date.
var (
	hashPattern = regexp.MustCompile(`^[0-9a-f]+$`)
	datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// Parse reads a git2(+subject) log from r and reduces it to a flat slice of
// model.Modification records, one per (commit, file) pair, in log order. The
// opts control input decoding; today only the encoding default is honored.
//
// Each blank-line-separated entry carries one or more prelude lines followed by
// numstat lines; only the last prelude supplies the rev/date/author/subject
// (matching code-maat's handling of stacked merge preludes). An entry with a
// prelude but no numstat lines (e.g. an empty merge) contributes no records.
func Parse(r io.Reader, _ model.Options) ([]model.Modification, error) {
	var mods []model.Modification
	entryNum := 0
	for lines, err := range tokenize(r) {
		if err != nil {
			return nil, err
		}
		entryNum++
		entryMods, err := parseEntry(lines)
		if err != nil {
			return nil, parseError(entryNum, err)
		}
		mods = append(mods, entryMods...)
	}
	if entryNum == 0 {
		return nil, ErrEmptyLog
	}
	return mods, nil
}

// parseError wraps a per-entry failure as the coded ErrParse, attaching the
// entry index and the offending source line (carried by an entryError) both as
// a human message and as structured details for the error envelope.
func parseError(entryNum int, err error) error {
	line := ""
	var ee *entryError
	if errors.As(err, &ee) {
		line = ee.line
	}
	return ErrParse.
		WithDetails(map[string]any{"entry": entryNum, "line": line}).
		Wrap(fmt.Errorf("entry %d, line %q: %w", entryNum, line, err))
}

// entryError pairs a parse failure with the offending source line so Parse can
// surface the line in the coded error's message and details.
type entryError struct {
	line string
	err  error
}

func (e *entryError) Error() string { return e.err.Error() }
func (e *entryError) Unwrap() error { return e.err }

// ParseString is a convenience wrapper over Parse for in-memory input, used
// chiefly by tests.
func ParseString(s string, opts model.Options) ([]model.Modification, error) {
	return Parse(strings.NewReader(s), opts)
}

// parseEntry turns one tokenized entry (its non-blank lines) into zero or more
// modification records. Leading lines beginning with "--" are prelude lines;
// the last one wins. Every remaining line is a numstat line.
func parseEntry(lines []string) ([]model.Modification, error) {
	var prelude string
	numstatStart := 0
	for i, line := range lines {
		if strings.HasPrefix(line, "--") {
			prelude = line
			numstatStart = i + 1
			continue
		}
		break
	}
	if prelude == "" {
		first := ""
		if len(lines) > 0 {
			first = lines[0]
		}
		return nil, &entryError{line: first, err: fmt.Errorf("expected prelude line beginning with %q", "--")}
	}

	rev, date, author, message, err := parsePrelude(prelude)
	if err != nil {
		return nil, &entryError{line: prelude, err: err}
	}

	numstat := lines[numstatStart:]
	if len(numstat) == 0 {
		return nil, nil
	}

	mods := make([]model.Modification, 0, len(numstat))
	for _, line := range numstat {
		mod, err := parseNumstat(line)
		if err != nil {
			return nil, &entryError{line: line, err: err}
		}
		mod.Rev = rev
		mod.Date = date
		mod.Author = author
		mod.Message = message
		mods = append(mods, mod)
	}
	return mods, nil
}

// parsePrelude splits a prelude line "--<hash>--<date>--<author>[--<subject>]"
// into its fields. The subject may itself contain "--", so every field from the
// fourth onward is rejoined; a missing subject defaults to "-".
func parsePrelude(line string) (rev, date, author, message string, err error) {
	fields := strings.Split(line, "--")
	// A leading "--" yields an empty first field, so fields[1:] are the values.
	if len(fields) < 4 {
		return "", "", "", "", fmt.Errorf("malformed prelude %q", line)
	}
	rev, date, author = fields[1], fields[2], fields[3]
	if !hashPattern.MatchString(rev) {
		return "", "", "", "", fmt.Errorf("malformed commit hash %q in prelude %q", rev, line)
	}
	if !datePattern.MatchString(date) {
		return "", "", "", "", fmt.Errorf("malformed date %q in prelude %q", date, line)
	}

	message = "-"
	if len(fields) > 4 {
		message = strings.Join(fields[4:], "--")
	}
	return rev, date, author, message, nil
}

// parseNumstat parses a "<added>\t<deleted>\t<path>" numstat line. A "-" count
// marks a binary file (both counts 0, Binary set); numeric counts populate the
// LOC fields. HasLoc is always set because a numstat line is present.
func parseNumstat(line string) (model.Modification, error) {
	fields := strings.SplitN(line, "\t", 3)
	if len(fields) != 3 {
		return model.Modification{}, fmt.Errorf("expected numstat, got %q", line)
	}

	added, deleted, path := fields[0], fields[1], fields[2]
	mod := model.Modification{Entity: path, HasLoc: true}

	if added == "-" || deleted == "-" {
		mod.Binary = true
		return mod, nil
	}

	var err error
	if mod.LocAdded, err = strconv.Atoi(added); err != nil {
		return model.Modification{}, fmt.Errorf("malformed added count %q in numstat %q", added, line)
	}
	if mod.LocDeleted, err = strconv.Atoi(deleted); err != nil {
		return model.Modification{}, fmt.Errorf("malformed deleted count %q in numstat %q", deleted, line)
	}
	return mod, nil
}
