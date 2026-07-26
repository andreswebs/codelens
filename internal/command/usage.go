package command

import (
	"strings"

	"github.com/andreswebs/codelens/internal/terr"
)

// The usage-error sentinels are the coded errors the classifier resolves a
// framework parse failure to. Each is an exit-64 usage error; the distinct code
// lets an agent react to the specific failure (an unknown/undefined flag, a bad
// flag value, or a missing required flag) rather than a single opaque usage
// code. They are registered via terr.New so they appear in terr.All() and thus
// in the schema error inventory (ADR 0003).
var (
	ErrUnknownFlag = terr.New(
		"unknown_flag", 64,
		"run `codelens <command> --help` to list valid flags",
		"unknown flag",
	)
	ErrInvalidValue = terr.New(
		"invalid_value", 64,
		"run `codelens <command> --help` for accepted flag values",
		"invalid flag value",
	)
	ErrMissingRequiredFlag = terr.New(
		"missing_required_flag", 64,
		"run `codelens <command> --help` to see required flags",
		"missing required flag",
	)
)

// usageMarkers maps substrings of the urfave/cli v3 parsing messages to the
// coded usage error each represents. Order is significant: the first matching
// marker wins, so more specific markers precede more general ones. Matching is
// substring (strings.Contains), not prefix: codelens's markers ("no such flag",
// "not set") appear mid-message, so prefix matching would silently miss them and
// reclassify the error as internal_error.
var usageMarkers = []struct {
	marker   string
	sentinel *terr.E
}{
	{"flag provided but not defined", ErrUnknownFlag},
	{"no such flag", ErrUnknownFlag},
	{"invalid value", ErrInvalidValue},
	{"Required flag", ErrMissingRequiredFlag},
	{"not set", ErrMissingRequiredFlag},
}

// classifyUsage reports the coded usage error err represents, or nil when err's
// message matches no known CLI-framework parsing marker. The returned error
// carries the matched sentinel's code, exit code, and hint but PRESERVES err's
// raw message, so the emitted envelope is byte-identical to when the classifier
// lived in internal/output. Unknown commands are classified upstream (they never
// reach the framework's flag parser) and so are not covered here.
func classifyUsage(err error) error {
	msg := err.Error()
	for _, c := range usageMarkers {
		if strings.Contains(msg, c.marker) {
			return terr.Newf(c.sentinel.Code(), c.sentinel.ExitCode(), c.sentinel.Hint(), "%s", msg)
		}
	}
	return nil
}

// usageClassCodes returns the distinct coded usage errors the classifier can
// produce, in first-seen order. It exposes the marker table as data so a
// conformance test can tie the classifier's codes to the registered sentinels.
func usageClassCodes() []string {
	seen := map[string]bool{}
	codes := make([]string, 0, len(usageMarkers))
	for _, c := range usageMarkers {
		if !seen[c.sentinel.Code()] {
			seen[c.sentinel.Code()] = true
			codes = append(codes, c.sentinel.Code())
		}
	}
	return codes
}
