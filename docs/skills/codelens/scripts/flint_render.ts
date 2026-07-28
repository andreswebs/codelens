/**
 * Render a Flint ChartAssemblyInput to SVG or PNG, headless.
 *
 * Reads a spec (as emitted by flint_spec.py) on stdin and writes the artifact
 * to stdout or -o. Renders in-process with no MCP client attached; this is
 * the only headless route to the ECharts-only templates (Network Graph).
 *
 *   codelens coupling < git.log | uv run flint_spec.py \
 *     | deno run --allow-env --allow-read --allow-sys --allow-ffi \
 *         --allow-write flint_render.ts --format png -o coupling.png
 *
 * Flags: [--backend vegalite|echarts] [--format svg|png] [--scale N]
 * [-o|--out FILE]. The backend defaults to the first one declaring the
 * spec's chartType, vegalite preferred; --scale multiplies PNG resolution.
 *
 * Always pass the permission flags shown (--allow-write only for -o): PNG
 * needs sys and ffi for the resvg native binding, and vega-lite SVG output
 * CHANGES when the ffi-loaded canvas module is unavailable (approximate text
 * metrics), so one canonical flag set keeps renders reproducible.
 *
 * Exit codes: 0 ok; 2 usage or a rejected spec.
 */

// UPGRADE CHECKLIST. Both imports below are pinned at 0.4.0. There is no
// contract test for any of this: the pin is re-verified BY HAND, which keeps
// JavaScript out of the test suite at the cost of not detecting drift until
// someone runs this list. Blast radius is bounded because flint_spec.py has no
// Flint dependency at all; only this renderer and the semantic-type and
// chart-type NAMES are exposed. Before changing either pin, re-verify:
//
// 1. All 9 target semantic types still resolve: Name, Category, Boolean, ID,
//    Date, Count, Quantity, Duration, Percentage.
// 2. Network Graph still reads x as source and y as target. Compile a
//    two-column edge table and inspect series[0].links.
// 3. _warnings still carries overflow entries on the assembled spec. The
//    truncation reporting below depends on that PRIVATE field.
// 4. intrinsicDomain is still a GATE rather than an override. If it becomes an
//    override, revisit flint_spec.py's bare `percentage` mapping, which exists
//    only because annotating it causes a 100x rendering error.
// 5. The 6 default chart-type names are unchanged.
// 6. The declared channel lists are unchanged and flint_spec.py's channel table
//    still matches them. That table is a COPY of Flint data no unit test can
//    verify, so this item is the ONLY scheduled parity check for it. The
//    TEMPLATE_CHANNELS validation below catches the same drift at render time,
//    but only for specs that are actually rendered.
//
// Why 0.4.0 and not latest: Deno's default minimumDependencyAge guard refuses a
// version published under 24 hours ago (verified against 0.4.1). Hitting that
// error on a bump is a supply-chain default doing its job, not a broken
// package; wait the window out rather than reflexively disabling the guard.
import { renderChart } from "npm:flint-chart-mcp@0.4.0/render";
import {
  ecGetTemplateChannels,
  vlGetTemplateChannels,
} from "npm:flint-chart@0.4.0";

const EXIT_USAGE = 2;

type Backend = "vegalite" | "echarts";

// Flint's own declared-channel lookups: the authoritative half of the F12
// validation, of which flint_spec.py's channel table is a copy. An unknown
// template yields an empty list, which is also how a backend is inferred.
const TEMPLATE_CHANNELS: Record<Backend, (chartType: string) => string[]> = {
  vegalite: vlGetTemplateChannels,
  echarts: ecGetTemplateChannels,
};

function die(msg: string): never {
  console.error(`flint_render.ts: ${msg}`);
  Deno.exit(EXIT_USAGE);
}

function resolveBackend(chartType: string, backendArg?: Backend): Backend {
  const candidates = backendArg
    ? [backendArg]
    : (["vegalite", "echarts"] as const);
  for (const backend of candidates) {
    if (TEMPLATE_CHANNELS[backend](chartType).length > 0) return backend;
  }
  die(
    backendArg
      ? `chart type ${
        JSON.stringify(chartType)
      } is not a ${backendArg} template`
      : `unknown chart type ${JSON.stringify(chartType)} on any backend`,
  );
}

// F12 layer 2: flint_spec.py already checked its own channel table, but that
// table is a copy; this check asks Flint itself, so a template that renames a
// channel fails loudly here on the first render after an upgrade.
function validateChannels(
  backend: Backend,
  chartType: string,
  encodings: Record<string, unknown>,
): void {
  const declared = TEMPLATE_CHANNELS[backend](chartType);
  const unknown = Object.keys(encodings).filter((ch) => !declared.includes(ch));
  if (unknown.length > 0) {
    die(
      `channel(s) ${unknown.join(", ")} not declared by ${backend} ` +
        `${JSON.stringify(chartType)} (declared: ${declared.join(", ")})`,
    );
  }
}

interface Cli {
  format: "svg" | "png";
  scale: number;
  backend?: Backend;
  out?: string;
}

function parseCli(argv: string[]): Cli {
  const cli: Cli = { format: "svg", scale: 1 };
  const args = [...argv];
  while (args.length > 0) {
    const flag = args.shift()!;
    switch (flag) {
      case "--format": {
        const value = args.shift();
        if (value !== "svg" && value !== "png") {
          die(`--format must be svg or png, got ${JSON.stringify(value)}`);
        }
        cli.format = value;
        break;
      }
      case "--backend": {
        const value = args.shift();
        if (value !== "vegalite" && value !== "echarts") {
          die(
            `--backend must be vegalite or echarts, got ${
              JSON.stringify(value)
            }`,
          );
        }
        cli.backend = value;
        break;
      }
      case "--scale": {
        const value = Number(args.shift());
        if (!Number.isFinite(value) || value <= 0) {
          die("--scale must be a positive number");
        }
        cli.scale = value;
        break;
      }
      case "-o":
      case "--out": {
        const value = args.shift();
        if (!value) die(`${flag} needs a file path`);
        cli.out = value;
        break;
      }
      default:
        die(`unknown flag ${JSON.stringify(flag)}`);
    }
  }
  return cli;
}

type ChartInput = Parameters<typeof renderChart>[0];

function loadSpec(text: string): ChartInput {
  let spec: unknown;
  try {
    spec = JSON.parse(text);
  } catch (e) {
    die(`invalid JSON on stdin: ${(e as Error).message}`);
  }
  if (spec === null || typeof spec !== "object" || Array.isArray(spec)) {
    die("input is not a ChartAssemblyInput (expected a JSON object)");
  }
  const chartSpec = (spec as Record<string, unknown>).chart_spec;
  if (
    chartSpec === null || typeof chartSpec !== "object" ||
    typeof (chartSpec as Record<string, unknown>).chartType !== "string"
  ) {
    die(
      "input carries no chart_spec.chartType; not a Flint spec " +
        "(emit one with flint_spec.py)",
    );
  }
  return spec as ChartInput;
}

const cli = parseCli(Deno.args);
const spec = loadSpec(await new Response(Deno.stdin.readable).text());
const chartSpec = spec.chart_spec as Record<string, unknown>;
const chartType = chartSpec.chartType as string;
const backend = resolveBackend(chartType, cli.backend);
validateChannels(
  backend,
  chartType,
  (chartSpec.encodings ?? {}) as Record<string, unknown>,
);

let result;
try {
  result = await renderChart(spec, backend, {
    format: cli.format,
    scale: cli.scale,
  });
} catch (e) {
  die(`flint-chart rejected the spec: ${(e as Error).message}`);
}
// The render result's warnings are the assembled spec's private _warnings
// field, where Flint records its own truncation (e.g. an overflow entry when
// the canvas budget drops rows). Re-emit each as a one-line JSON diagnostic,
// matching how codelens reports warnings on stderr.
for (const warning of result.warnings) {
  console.error(JSON.stringify(warning));
}
const artifact: Uint8Array = cli.format === "svg"
  ? new TextEncoder().encode(result.svg!)
  : new Uint8Array(result.buffer!);
if (cli.out) {
  await Deno.writeFile(cli.out, artifact);
} else {
  // Deno.stdout.write may short-write on a pipe; loop until drained.
  let written = 0;
  while (written < artifact.length) {
    written += await Deno.stdout.write(artifact.subarray(written));
  }
}
console.error(
  `rendered ${chartType} (${backend}) ${cli.format}: ` +
    `${result.width}x${result.height}`,
);
