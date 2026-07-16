# Composing a findings report

How `scripts/report.py` turns a full codelens run into one sequenced findings
report. Loaded from [SKILL.md](../SKILL.md) step 6. The report is plain markdown
meant to be **read** (rendered with pandoc or any HTML markdown pipeline) - there is
no MARP and no slide deck.

The script pins the structure, the order, the figure embedding, and the misuse
guardrails; the agent supplies the prose in a findings file. So the report is
reproducible while the reading stays a matter of judgment
([interpretation.md](interpretation.md) is the reading authority).

## Pipeline

1. Run the analyses and render the **degraded static** figures (SVG) into one
   directory. `scripts/run.bash` does this in one command:

   ```sh
   bash scripts/run.bash --repo PATH --out out/
   ```

   It generates the logs, runs every analysis, renders the figures under
   `out/figs/` with the conventional stems below, and writes `out/digest.md`
   (step 2's grounding). It anchors a 12-month window to the repo's last commit
   (`--months N` to resize, `--full-history` for stale or front-loaded repos),
   applies a built-in generated-file exclude set to every entity-centric analysis
   (`--exclude GLOB` to add more), and runs `code-age` against full history. It is
   read-only against the repo and best-effort: a figure with no data (for example
   coupling below the threshold) is skipped, not fatal. Requires codelens, git,
   tokei, and uv on PATH.

   To render by hand instead, run `treemap.py` for the enclosure family,
   `pair_matrix.py` for coupling and communication, and the static charts, writing
   the conventional stems below. A missing figure just omits its picture.
2. Write the findings file (below), one prose block per analysis, grounded in
   `out/digest.md`: a compact per-analysis signal (hotspots split code vs
   docs/config, coupling, ownership, fragmentation, age, churn, vocabulary) that
   `run.bash` writes so the reading never depends on opening the multi-megabyte
   JSON. `scripts/digest.py <out>` regenerates it standalone.
3. Assemble:

   ```sh
   uv run scripts/report.py --findings findings.md --figures-dir out/figs/ \
     --summary out/summary.json -o report.md
   ```

At least one of `--findings`, `--figures-dir`, `--summary` must be present
(otherwise exit 3, nothing to assemble).

## Fixed sequence (the investigative funnel, business-first)

The report always has these eleven `##` sections, in this order (see the funnel in
[interpretation.md](interpretation.md)):

1. Executive summary - business framing; the situational-awareness tiles from
   `--summary` render here.
2. Hotspots - where the risk is.
3. Complexity trend - is it getting worse?
4. Change coupling - why changes ripple.
5. Knowledge & ownership _(social)_.
6. Fragmentation _(social)_.
7. Communication & Conway alignment _(social)_.
8. Code age - stabilization.
9. Churn - the macro trend.
10. Commit vocabulary.
11. Recommended actions - accept / prioritise low-risk / mitigate.

## Findings file

Plain markdown. The `# H1` line is the report title, and its only source: a
`## title` block is ignored, so the title is never overridden or blanked. Each
other `## <key>` line starts a block whose body (free markdown prose) is slotted
into the matching section; keys are normalized (lowercased, spaces/hyphens to
underscores). An absent block renders a neutral placeholder, so a partial findings
file never breaks the run.

Reserved keys: `window`, `executive_summary`, `hotspots`, `complexity_trend`,
`coupling`, `knowledge`, `fractal`, `communication`, `code_age`, `churn`,
`word_cloud`, `risk_choices`.

```markdown
# keeper-core evolutionary analysis
## window
last 12 months (2025-07 to 2026-07)
## executive_summary
Change concentrates in the disbursement services; the rest is calm.
## hotspots
`EmployeeStatusService.cs` is the real offender (199 revisions, small, hot).
## risk_choices
Mitigate the disbursement hotspot first; accept the stable config churn.
```

Gotcha: some host environments run a hook that guards the Write tool on files named
`findings.md` or report filenames. If a write is blocked, write the file via a shell
heredoc, or write to a differently-named file and rename it. This is host-environment
behavior, not a codelens rule.

### Writing a findings block

The prose is the judgment the report cannot pin. Write each block from
`out/digest.md` (open the raw JSON only for a number the digest omits):

- **Ground every claim in the digest's numbers.** Name the specific file, coupled
  pair, author, or fractal/degree value with its metric; no generic filler.
- **Be honest about thin signal.** If coupling is "none above threshold", or there
  are only a few commits or authors, say the signal is thin and why. Do not invent
  findings; a short accurate block beats a padded one.
- **Separate generated from authored.** If hotspots or churn are dominated by
  generated or reference data that slipped the excludes, say so rather than reading
  it as real code risk (see the reference-data note in
  [operating.md](operating.md)).
- **`risk_choices`** is three bullets, `Accept:`, `Prioritise now:`, and
  `Mitigate over time:`, each naming specific files or areas.
- Keep each block tight (2 to 6 sentences), read per
  [interpretation.md](interpretation.md), and honour its guardrails: never rank
  individuals, ownership and communication are probabilistic key-person and
  coordination risk, and the commit word cloud is heuristic only.

This mirrors the digest-first, subagent-friendly workflow used to write findings at
fleet scale: one agent per repo, grounded in that repo's `digest.md`.

## Figures

`--figures-dir` is scanned for these stems (`.svg`), one per section:

`summary` `hotspots` `complexity` `coupling` `knowledge` `fractal` `network`
`age` `churn` `cloud`.

Each present figure is embedded **inline** (`<svg>...</svg>`, XML prolog and doctype
stripped) so the report is a single self-contained file with no external asset
references. Caveat: inline SVG renders under pandoc and HTML pipelines, but GitHub's
markdown sanitizer strips it - the report targets HTML/pandoc rendering, not GitHub
preview.

When scripting the figures yourself, judge each render script by its **exit code**,
not by stderr: the scripts print their `wrote ...` summary and uv's `Installed N
packages` to stderr on success, so a non-empty stderr is not a failure.
`scripts/run.bash` relies on the exit code.

## Guardrails (always emitted, cannot be omitted)

Per the misuse guardrails in [interpretation.md](interpretation.md), `report.py`
injects, regardless of the findings:

- a "not for performance evaluation" disclaimer on every social section (knowledge,
  fragmentation, communication);
- a team-aggregation note on the communication section (an inter-team tie is a
  _potential_ bottleneck, not proof);
- a "heuristic only" label on the commit-vocabulary section.
