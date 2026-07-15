// Package pipeline composes the optional transform stages that sit between the
// git-log parser and an analysis. It applies grouping, temporal windowing, and
// team mapping in the fixed order code-maat uses (group -> temporal -> teammap),
// with each stage a no-op when its configuration is absent, so every analysis
// honors --group, --temporal-period, and --team-map uniformly.
package pipeline

import (
	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/transform/group"
	"github.com/andreswebs/codelens/internal/transform/teammap"
	"github.com/andreswebs/codelens/internal/transform/temporal"
)

// Config selects which transform stages run. Each field's zero value disables
// its stage: a nil/empty GroupSpecs skips grouping, a TemporalPeriod of 0 skips
// windowing, and a nil/empty TeamMap skips team substitution.
type Config struct {
	// GroupSpecs are the compiled layer-mapping rules; nil or empty skips
	// grouping. An empty rule set is treated as "no grouping requested" rather
	// than "drop every entity".
	GroupSpecs []group.Spec
	// TemporalPeriod is the sliding-window size in days; 0 (or negative) skips
	// temporal collapsing.
	TemporalPeriod int
	// TeamMap is the author->team lookup; nil or empty skips substitution.
	TeamMap map[string]string
}

// Apply runs the enabled stages over mods in code-maat's canonical order:
// grouping first (so windowing and team metrics see layer-level entities), then
// temporal windowing, then team mapping. When cfg enables no stage the input is
// returned unchanged. The input slice is never mutated; each active stage
// returns a fresh slice.
func Apply(mods []model.Modification, cfg Config) ([]model.Modification, error) {
	out := mods
	if len(cfg.GroupSpecs) > 0 {
		out = group.Apply(out, cfg.GroupSpecs)
	}
	if cfg.TemporalPeriod > 0 {
		windowed, err := temporal.Apply(out, cfg.TemporalPeriod)
		if err != nil {
			return nil, err
		}
		out = windowed
	}
	if len(cfg.TeamMap) > 0 {
		out = teammap.Apply(out, cfg.TeamMap)
	}
	return out, nil
}
