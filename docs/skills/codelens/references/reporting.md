# Composing a findings report

How `scripts/report.py` turns a full codelens run into one sequenced findings
report. Loaded from [SKILL.md](../SKILL.md) step 6. The report is plain markdown
meant to be **read** (rendered with pandoc or any HTML markdown pipeline) — there is
no MARP and no slide deck.

The script pins the structure, the order, the figure embedding, and the misuse
guardrails; the agent supplies the prose in a findings file. So the report is
reproducible while the reading stays a matter of judgment
([interpretation.md](interpretation.md) is the reading authority).

## Pipeline

1. Run the analyses and render the **degraded static** figures (SVG) into one
   directory with the conventional names below (`treemap.py` for the enclosure
   family, `pair_matrix.py` for coupling and communication, plus the existing
   static charts). A missing figure just omits its picture.
2. Write the findings file (below), one prose block per analysis.
3. Assemble:

   ```sh
   uv run scripts/report.py --findings findings.md --figures-dir figs/ \
     --summary summary.json -o report.md
   ```

At least one of `--findings`, `--figures-dir`, `--summary` must be present
(otherwise exit 3, nothing to assemble).

## Fixed sequence (the investigative funnel, business-first)

The report always has these eleven `##` sections, in this order (see the funnel in
[interpretation.md](interpretation.md)):

1. Executive summary — business framing; the situational-awareness tiles from
   `--summary` render here.
2. Hotspots — where the risk is.
3. Complexity trend — is it getting worse?
4. Change coupling — why changes ripple.
5. Knowledge & ownership _(social)_.
6. Fragmentation _(social)_.
7. Communication & Conway alignment _(social)_.
8. Code age — stabilization.
9. Churn — the macro trend.
10. Commit vocabulary.
11. Recommended actions — accept / prioritise low-risk / mitigate.

## Findings file

Plain markdown. A `# H1` line is the report title. Each `## <key>` line starts a
block whose body (free markdown prose) is slotted into the matching section; keys
are normalized (lowercased, spaces/hyphens to underscores). An absent block renders
a neutral placeholder, so a partial findings file never breaks the run.

Reserved keys: `title`, `window`, `executive_summary`, `hotspots`,
`complexity_trend`, `coupling`, `knowledge`, `fractal`, `communication`,
`code_age`, `churn`, `word_cloud`, `risk_choices`.

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

## Figures

`--figures-dir` is scanned for these stems (`.svg`), one per section:

`summary` `hotspots` `complexity` `coupling` `knowledge` `fractal` `network`
`age` `churn` `cloud`.

Each present figure is embedded **inline** (`<svg>...</svg>`, XML prolog and doctype
stripped) so the report is a single self-contained file with no external asset
references. Caveat: inline SVG renders under pandoc and HTML pipelines, but GitHub's
markdown sanitizer strips it — the report targets HTML/pandoc rendering, not GitHub
preview.

## Guardrails (always emitted, cannot be omitted)

Per the misuse guardrails in [interpretation.md](interpretation.md), `report.py`
injects, regardless of the findings:

- a "not for performance evaluation" disclaimer on every social section (knowledge,
  fragmentation, communication);
- a team-aggregation note on the communication section (an inter-team tie is a
  _potential_ bottleneck, not proof);
- a "heuristic only" label on the commit-vocabulary section.
