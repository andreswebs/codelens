package command

import (
	"regexp"
	"testing"

	"github.com/andreswebs/codelens/internal/terr"
)

// wantSentinelCount is the number of registering terr.New sentinels linked into
// the whole binary. Re-derive it from the tree rather than trusting the number:
// grep -rn 'terr\.New(' --include='*.go' . | grep -v _test.go. Current
// derivation (22):
//
//	internal/gitlog/errors.go              3 (parse_error, empty_log, invalid_control_char)
//	internal/output/fields.go              1 (invalid_field)
//	internal/output/format.go              1 (unknown_format)
//	internal/analysis/messages.go          2 (invalid_expression, missing_messages)
//	internal/analysis/codeage.go           1 (invalid_time_now)
//	internal/analysis/churn/churn.go       1 (missing_metrics)
//	internal/transform/temporal/temporal.go 2 (invalid_temporal_period, invalid_temporal_date)
//	internal/transform/group/group.go      1 (invalid_group)
//	internal/transform/teammap/teammap.go  1 (invalid_team_map)
//	internal/transform/filter/filter.go    1 (invalid_glob)
//	internal/command/errors.go             5 (unknown_command, log_open_failed,
//	                                          input_file_open_failed, unknown_schema_command,
//	                                          invalid_after_date)
//	internal/command/usage.go              3 (unknown_flag, invalid_value, missing_required_flag)
//
// The per-file tallies sum to 22. cod-q42s raised this from 18 by promoting the
// formerly-inline unknown_command to a registered sentinel and adding the three
// classifier sentinels.
const wantSentinelCount = 22

// snakeCase is the ADR 0003 code shape: lowercase words joined by underscores.
var snakeCase = regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)

// allowedExitCodes is the set ADR 0002 lets codelens produce, derived from the
// shared ExitCodes registry (registry.go) so it cannot drift from it.
var allowedExitCodes = func() map[int]bool {
	m := make(map[int]bool, len(ExitCodes))
	for _, c := range ExitCodes {
		m[c] = true
	}
	return m
}()

func TestRegistry_Coherent(t *testing.T) {
	all := terr.All()

	if len(all) != wantSentinelCount {
		t.Errorf("terr.All() returned %d sentinels, want %d (re-derive from the tree; see comment)", len(all), wantSentinelCount)
	}

	seen := make(map[string]bool, len(all))
	for _, e := range all {
		code := e.Code()
		if code == "" {
			t.Errorf("sentinel with exit %d has an empty code", e.ExitCode())
			continue
		}
		if seen[code] {
			t.Errorf("duplicate error code %q in the registry", code)
		}
		seen[code] = true

		if !snakeCase.MatchString(code) {
			t.Errorf("error code %q is not snake_case", code)
		}
		if !allowedExitCodes[e.ExitCode()] {
			t.Errorf("code %q has exit %d, not in {0,64,65,70,74}", code, e.ExitCode())
		}
	}
}
