package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/andreswebs/codelens/internal/analysis"
	"github.com/andreswebs/codelens/internal/gitlog"
	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/output"
	"github.com/andreswebs/codelens/internal/pipeline"
	"github.com/andreswebs/codelens/internal/transform/filter"
	"github.com/andreswebs/codelens/internal/transform/group"
	"github.com/andreswebs/codelens/internal/transform/teammap"
	"github.com/urfave/cli/v3"
)

// globalFlags builds the flags shared by every analysis subcommand. They are
// registered on the root command (not Local), so urfave inherits them into each
// subcommand's flag set and resolves them via cmd's lineage regardless of
// whether they appear before or after the subcommand name.
func globalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:  "log",
			Usage: "read the git log from `FILE` (default stdin; \"-\" forces stdin)",
		},
		&cli.StringFlag{
			Name:  "input-encoding",
			Value: "UTF-8",
			Usage: "character `ENC`oding of the log input",
		},
		&cli.StringFlag{
			Name:  "fields",
			Usage: "comma-separated JSON field `PATHS` to project",
		},
		&cli.IntFlag{
			Name:  "rows",
			Usage: "cap output to `N` rows after sorting (0 = all)",
		},
		&cli.StringSliceFlag{
			Name:  "include",
			Usage: "keep only entities matching `GLOB` (gitignore-style, repeatable; exclude wins)",
		},
		&cli.StringSliceFlag{
			Name:  "exclude",
			Usage: "drop entities matching `GLOB` (gitignore-style, repeatable; applied after --include)",
		},
		&cli.StringFlag{
			Name:  "group",
			Usage: "layer-mapping `FILE`",
		},
		&cli.StringFlag{
			Name:  "group-format",
			Value: "text",
			Usage: "format of --group: text|json",
		},
		&cli.StringFlag{
			Name:  "team-map",
			Usage: "author-to-team map `FILE`",
		},
		&cli.StringFlag{
			Name:  "team-map-format",
			Value: "csv",
			Usage: "format of --team-map: csv|json",
		},
		&cli.IntFlag{
			Name:  "temporal-period",
			Usage: "collapse commits into sliding `N`-day change sets (0 = off)",
		},
	}
}

// analysisCommands builds one cli.Command per registered analysis descriptor,
// wiring its aliases, summary, per-analysis flags, and an Action that reads the
// log (from stdin or --log), parses it, runs the analysis, applies --rows
// truncation, and emits the result. Global flags come from the root command.
func analysisCommands(stdin io.Reader) []*cli.Command {
	descriptors := analysis.All()
	cmds := make([]*cli.Command, 0, len(descriptors))
	for _, d := range descriptors {
		cmds = append(cmds, &cli.Command{
			Name:    d.Name,
			Aliases: d.Aliases,
			Usage:   d.Summary,
			Flags:   perCommandFlags(d),
			Action:  actionFor(d, stdin),
		})
	}
	return cmds
}

// perCommandFlags converts a descriptor's declared flags into cli flags. Only
// the flags an analysis declares are attached to its command, so a flag never
// no-ops on a command that ignores it.
func perCommandFlags(d analysis.Descriptor) []cli.Flag {
	flags := make([]cli.Flag, 0, len(d.Flags))
	for _, f := range d.Flags {
		flags = append(flags, toCLIFlag(f))
	}
	return flags
}

// toCLIFlag maps an analysis.Flag to its concrete cli.Flag by declared type. An
// unknown type is a programmer error in a descriptor and panics at startup.
func toCLIFlag(f analysis.Flag) cli.Flag {
	switch f.Type {
	case "int":
		def, _ := f.Default.(int)
		return &cli.IntFlag{Name: f.Name, Value: def, Usage: f.Desc, Required: f.Required}
	case "bool":
		def, _ := f.Default.(bool)
		return &cli.BoolFlag{Name: f.Name, Value: def, Usage: f.Desc, Required: f.Required}
	case "string":
		def, _ := f.Default.(string)
		return &cli.StringFlag{Name: f.Name, Value: def, Usage: f.Desc, Required: f.Required}
	default:
		panic(fmt.Sprintf("analysis flag %q has unsupported type %q", f.Name, f.Type))
	}
}

// actionFor returns the cli.ActionFunc that runs descriptor d. The parsed
// modifications flow log -> parse -> pipeline (filter -> group -> temporal ->
// team-map) -> analysis -> truncate -> emit. The pipeline stages are each
// skipped unless their flag is supplied, so every analysis honors
// --include/--exclude, --group, --temporal-period, and --team-map without
// wiring them per command.
func actionFor(d analysis.Descriptor, stdin io.Reader) cli.ActionFunc {
	return func(_ context.Context, cmd *cli.Command) error {
		r, closeLog, err := openLog(cmd, stdin)
		if err != nil {
			return err
		}
		defer closeLog()

		mods, err := gitlog.Parse(r, model.Options{InputEncoding: cmd.String("input-encoding")})
		if err != nil {
			return err
		}

		cfg, err := pipelineConfig(cmd)
		if err != nil {
			return err
		}
		mods, err = pipeline.Apply(mods, cfg)
		if err != nil {
			return err
		}

		warnTemporalRecounts(cmd, d)

		opts := analysisOpts(cmd, d)
		opts.Warn = func(code, message, hint string, details any) {
			output.EmitWarning(cmd.Root().ErrWriter, code, message, hint, details)
		}

		rows, err := d.Run(mods, opts)
		if err != nil {
			return err
		}

		res := output.NewResult(output.Meta{
			Analysis:   d.Name,
			Shape:      string(d.Shape),
			Semantics:  semanticsFor(cmd, d),
			Transforms: transformsRecord(cmd),
			Columns:    columnNames(d),
		}, rows)
		res.Params = effectiveParams(cmd, d)
		truncate(&res, cmd.Int("rows"))

		return output.EmitProjected(cmd.Root().Writer, res, cmd.String("fields"), columnNames(d))
	}
}

// pipelineConfig assembles the transform configuration from the global flags:
// compiling the --include/--exclude globs, parsing the --group and --team-map
// definition files (each in the format its *-format flag selects), and reading
// --temporal-period. Absent flags leave their stage disabled. A malformed glob
// or definition surfaces the transform's own coded error (a usage error, exit
// 64, or a data error, exit 65); an unreadable file is an I/O error (exit 74).
func pipelineConfig(cmd *cli.Command) (pipeline.Config, error) {
	var cfg pipeline.Config

	spec, err := filter.Compile(cmd.StringSlice("include"), cmd.StringSlice("exclude"))
	if err != nil {
		return pipeline.Config{}, err
	}
	cfg.FilterSpec = spec

	if path := cmd.String("group"); path != "" {
		specs, err := parseDefinition(path, "group", func(r io.Reader) ([]group.Spec, error) {
			return group.Parse(r, cmd.String("group-format"))
		})
		if err != nil {
			return pipeline.Config{}, err
		}
		cfg.GroupSpecs = specs
	}

	cfg.TemporalPeriod = cmd.Int("temporal-period")

	if path := cmd.String("team-map"); path != "" {
		teams, err := parseDefinition(path, "team-map", func(r io.Reader) (map[string]string, error) {
			return teammap.Parse(r, cmd.String("team-map-format"))
		})
		if err != nil {
			return pipeline.Config{}, err
		}
		cfg.TeamMap = teams
	}

	return cfg, nil
}

// parseDefinition opens the read-only definition file at path and hands it to
// parse. A failure to open the file is wrapped as an I/O error tagged with the
// originating flag; parse errors are returned as-is so each transform's coded
// error reaches the top level unchanged.
func parseDefinition[T any](path, flag string, parse func(io.Reader) (T, error)) (T, error) {
	var zero T
	f, err := os.Open(path) // #nosec G304 -- read-only, user-supplied definition input
	if err != nil {
		return zero, errFileOpen.
			WithDetails(map[string]string{"flag": flag, "path": path}).
			Wrap(err)
	}
	defer func() { _ = f.Close() }()
	return parse(f)
}

// openLog resolves the analysis input: stdin when --log is empty or "-", else
// the named file opened read-only. The returned close function is always safe
// to call and is a no-op for stdin.
func openLog(cmd *cli.Command, stdin io.Reader) (io.Reader, func(), error) {
	path := cmd.String("log")
	if path == "" || path == "-" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(path) // #nosec G304 -- read-only, user-supplied analysis input
	if err != nil {
		return nil, func() {}, errLogOpen.
			WithDetails(map[string]string{"path": path}).
			Wrap(err)
	}
	return f, func() { _ = f.Close() }, nil
}

// analysisOpts collects the effective per-analysis options from the parsed
// flags. Only the flags the descriptor declares are read, so a command never
// picks up a threshold that belongs to a different analysis.
func analysisOpts(cmd *cli.Command, d analysis.Descriptor) analysis.Opts {
	declared := make(map[string]bool, len(d.Flags))
	for _, f := range d.Flags {
		declared[f.Name] = true
	}

	var o analysis.Opts
	if declared["min-revs"] {
		o.MinRevs = cmd.Int("min-revs")
	}
	if declared["min-shared-revs"] {
		o.MinSharedRevs = cmd.Int("min-shared-revs")
	}
	if declared["min-coupling"] {
		o.MinCoupling = cmd.Int("min-coupling")
	}
	if declared["max-coupling"] {
		o.MaxCoupling = cmd.Int("max-coupling")
	}
	if declared["max-changeset-size"] {
		o.MaxChangesetSize = cmd.Int("max-changeset-size")
	}
	if declared["verbose"] {
		o.Verbose = cmd.Bool("verbose")
	}
	if declared["time-now"] {
		o.TimeNow = cmd.String("time-now")
	}
	if declared["expression"] {
		o.Expression = cmd.String("expression")
	}
	return o
}

// effectiveParams echoes the effective value of every flag the descriptor
// declares, keyed by flag name, so a result documents the thresholds actually
// applied (defaults included). It mirrors analysisOpts's declared-flag pattern.
// A flagless analysis returns nil so params is omitted from its envelope,
// keeping that output byte-identical.
func effectiveParams(cmd *cli.Command, d analysis.Descriptor) map[string]any {
	if len(d.Flags) == 0 {
		return nil
	}
	params := make(map[string]any, len(d.Flags))
	for _, f := range d.Flags {
		switch f.Type {
		case "int":
			params[f.Name] = cmd.Int(f.Name)
		case "bool":
			params[f.Name] = cmd.Bool(f.Name)
		case "string":
			params[f.Name] = cmd.String(f.Name)
		}
	}
	return params
}

// columnNames returns the descriptor's declared payload field names in order.
// It is no longer a csv/table header source (that surface is gone): it seeds
// output.Meta.Columns and the --fields valid-path set, so a projection like
// --fields rows.entity stays valid even when the payload is empty.
func columnNames(d analysis.Descriptor) []string {
	names := make([]string, 0, len(d.RowSchema))
	for _, c := range d.RowSchema {
		names = append(names, c.Name)
	}
	return names
}

// semanticsFor builds the envelope's field-to-semantic map for this run: the
// descriptor's declared semantics minus the columns excluded by an unset flag
// (D15), then adjusted for the active transforms (D4). It is a pure function of
// the descriptor and the parsed flags, so the map is deterministic for a given
// command line. The result is converted to plain strings at the return: output
// does not import analysis (the dependency runs the other way), so the typed
// vocabulary stops here at the boundary.
func semanticsFor(cmd *cli.Command, d analysis.Descriptor) map[string]string {
	omit := make(map[string]bool)
	for _, c := range d.RowSchema {
		if c.FlagGated != "" && !cmd.Bool(c.FlagGated) {
			omit[c.FlagGated] = true
		}
	}
	sem := adjustForTransforms(analysis.SemanticsOf(d, omit), cmd.String("group") != "")
	out := make(map[string]string, len(sem))
	for name, s := range sem {
		out[name] = string(s)
	}
	return out
}

// adjustForTransforms returns semantics describing what THIS RUN emitted. A
// transform that destroys a field's structural affordance degrades its semantic:
// --group replaces entity paths with layer names, so a filepath (splittable on
// "/") becomes an opaque label. A transform that merely aggregates does NOT:
// --team-map replaces an author with a team, and both are opaque categorical
// actor names, so person stands (that is why only filepath is degraded here).
//
// This is why the schema and the envelope can disagree: the schema declares the
// command's default (filepath), the envelope declares the invocation (label).
// The rule is applied generically over the map rather than naming entity, so
// coupling's coupled column is covered too.
func adjustForTransforms(sem map[string]analysis.Semantic, grouped bool) map[string]analysis.Semantic {
	if !grouped {
		return sem
	}
	out := make(map[string]analysis.Semantic, len(sem))
	for name, s := range sem {
		if s == analysis.SemanticFilepath {
			s = analysis.SemanticLabel
		}
		out[name] = s
	}
	return out
}

// warnTemporalRecounts raises temporal_period_recounts when --temporal-period
// distorts an analysis's additive columns: the transform reinterprets a
// revision as a logical change set, so a count column tallies overlapping
// windows rather than commits. It lives here rather than in an analysis because
// Opts deliberately excludes transform state (the pipeline runs before Run);
// only the command layer holds both the descriptor and the parsed flags.
//
// A changeset-based analysis (coupling, sum-of-coupling) is exempt: treating a
// revision as a logical change set is the entire purpose of the flag there, so
// under a naive additive-column rule the two analyses the flag exists to serve
// would be the loudest warners. The affected columns are named in details so a
// consumer branches on data rather than parsing prose. Like every warning, it
// never alters the exit code, and the transform's numbers are unchanged.
func warnTemporalRecounts(cmd *cli.Command, d analysis.Descriptor) {
	period := cmd.Int("temporal-period")
	if period <= 0 || d.ChangesetBased {
		return
	}
	affected := make([]string, 0, len(d.RowSchema))
	for _, c := range d.RowSchema {
		if analysis.AggRoleOf(c.Semantic) == analysis.AggAdditive {
			affected = append(affected, c.Name)
		}
	}
	if len(affected) == 0 {
		return
	}
	output.EmitWarning(cmd.Root().ErrWriter,
		"temporal_period_recounts",
		"counts are per sliding window, not per commit",
		"--temporal-period reinterprets a revision as a logical change set, so the named columns count windows rather than commits; drop it for commit-accurate counts, or use it with coupling / sum-of-coupling where that is the intent",
		map[string]any{
			"period_days":      period,
			"affected_columns": affected,
			"analysis":         d.Name,
		},
	)
}

// transformsRecord records which pipeline transforms actually ran (D4b), keyed in
// snake_case to match every other envelope key even though the flags are
// kebab-case (params keeps its flag-name keying for compatibility). It returns nil
// when the pipeline was a pass-through, so the transforms key is omitted entirely.
// group and team_map are booleans rather than paths: a local filesystem path is
// noise for a consumer and leaks the caller's layout.
func transformsRecord(cmd *cli.Command) map[string]any {
	t := make(map[string]any)
	if inc := cmd.StringSlice("include"); len(inc) > 0 {
		t["include"] = inc
	}
	if exc := cmd.StringSlice("exclude"); len(exc) > 0 {
		t["exclude"] = exc
	}
	if cmd.String("group") != "" {
		t["group"] = true
	}
	if p := cmd.Int("temporal-period"); p != 0 {
		t["temporal_period"] = p
	}
	if cmd.String("team-map") != "" {
		t["team_map"] = true
	}
	if len(t) == 0 {
		return nil
	}
	return t
}

// truncate caps res to its first n rows after the analysis's own sort, setting
// the truncation metadata (row_count, total_count, truncated) so an agent can
// tell a capped result from a complete one. n <= 0 means "all rows" and a cap
// at or beyond the row count is a no-op. It operates on res.Payload: a non-slice
// payload (a future tree or graph) has RowLen 0 and is left alone.
func truncate(res *output.Result, n int) {
	if n <= 0 {
		return
	}
	total := output.RowLen(res.Payload)
	if total == 0 || n >= total {
		return
	}
	res.Payload = reflect.ValueOf(res.Payload).Slice(0, n).Interface()
	res.RowCount = n
	res.TotalCount = total
	res.Truncated = true
}
