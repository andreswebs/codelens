package main

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
	"github.com/andreswebs/codelens/internal/terr"
	"github.com/andreswebs/codelens/internal/transform/group"
	"github.com/andreswebs/codelens/internal/transform/teammap"
	"github.com/urfave/cli/v3"
)

// errLogOpen marks a failure to open the file named by --log. It is an input
// error (exit 3): the path is user-supplied, so an unreadable file is never an
// internal fault.
var errLogOpen = terr.New(
	"input_error", 3,
	"check that the --log path exists and is readable",
	"cannot open log file",
)

// errFileOpen marks a failure to open a user-supplied auxiliary file (--group
// or --team-map). Like --log, an unreadable path is an input error (exit 3): the
// path is user-supplied, so an I/O failure is never an internal fault. The
// offending flag and path are attached as details.
var errFileOpen = terr.New(
	"input_error", 3,
	"check that the file path exists and is readable",
	"cannot open input file",
)

// globalFlags builds the flags shared by every analysis subcommand. They are
// registered on the root command (not Local), so urfave inherits them into each
// subcommand's flag set and resolves them via cmd's lineage regardless of
// whether they appear before or after the subcommand name. format and debug are
// bound to the caller's destinations so the top level can render errors and gate
// diagnostics even when a subcommand's Action never runs.
func globalFlags(format *string, debug *bool) []cli.Flag {
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
			Name:        "format",
			Value:       "json",
			Usage:       "output `FMT`: json|ndjson|csv|table",
			Destination: format,
		},
		&cli.StringFlag{
			Name:  "fields",
			Usage: "comma-separated JSON field `PATHS` to project (json only)",
		},
		&cli.IntFlag{
			Name:  "rows",
			Usage: "cap output to `N` rows after sorting (0 = all)",
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
		&cli.BoolFlag{
			Name:        "debug",
			Usage:       "emit verbose diagnostics to stderr",
			Destination: debug,
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
			Name:         d.Name,
			Aliases:      d.Aliases,
			Usage:        d.Summary,
			Flags:        perCommandFlags(d),
			Action:       actionFor(d, stdin),
			OnUsageError: onUsageError,
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
// modifications flow log -> parse -> pipeline (group -> temporal -> team-map) ->
// analysis -> truncate -> emit. The pipeline stages are each skipped unless
// their flag is supplied, so every analysis honors --group, --temporal-period,
// and --team-map without wiring them per command.
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

		rows, err := d.Run(mods, analysisOpts(cmd, d))
		if err != nil {
			return err
		}

		res := output.NewResult(d.Name, rows)
		res.Params = effectiveParams(cmd, d)
		truncate(&res, cmd.Int("rows"))

		return output.Emit(cmd.Root().Writer, cmd.String("format"), res, columnNames(d), cmd.String("fields"))
	}
}

// pipelineConfig assembles the transform configuration from the global flags,
// parsing the --group and --team-map definition files (each in the format its
// *-format flag selects) and reading --temporal-period. Absent flags leave their
// stage disabled. A malformed definition surfaces the transform's own coded
// error; an unreadable file is an input error (exit 3).
func pipelineConfig(cmd *cli.Command) (pipeline.Config, error) {
	var cfg pipeline.Config

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
// parse. A failure to open the file is wrapped as an input error tagged with the
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

// columnNames extracts the ordered snake_case row-schema column names from a
// descriptor, the header/order source for the csv and table formats.
func columnNames(d analysis.Descriptor) []string {
	names := make([]string, len(d.RowSchema))
	for i, c := range d.RowSchema {
		names[i] = c.Name
	}
	return names
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

// truncate caps res to its first n rows after the analysis's own sort, setting
// the truncation metadata (row_count, total_count, truncated) so an agent can
// tell a capped result from a complete one. n <= 0 means "all rows" and a cap
// at or beyond the row count is a no-op.
func truncate(res *output.Result, n int) {
	if n <= 0 {
		return
	}
	total := output.RowLen(res.Rows)
	if total == 0 || n >= total {
		return
	}
	res.Rows = reflect.ValueOf(res.Rows).Slice(0, n).Interface()
	res.RowCount = n
	res.TotalCount = total
	res.Truncated = true
}
