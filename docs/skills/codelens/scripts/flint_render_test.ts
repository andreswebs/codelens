/**
 * Behavioral tests for flint_render.ts, through its rendered output.
 *
 * Tests run the script as a subprocess, mirroring flint_spec_test.py: they
 * pipe a Flint ChartAssemblyInput on stdin and assert on stdout bytes, the
 * stderr diagnostics, and the exit code. The fixture specs are the recorded
 * outputs flint_spec.py emits for the six per-analysis overrides.
 *
 * Run: deno test --allow-run --allow-read --allow-write --allow-env flint_render_test.ts
 */

import { assert, assertEquals } from "jsr:@std/assert@1";

const SCRIPT = new URL("flint_render.ts", import.meta.url).pathname;
// --allow-write is only needed for -o; the rest is the canonical render set.
const PERMISSIONS = [
  "--allow-env",
  "--allow-read",
  "--allow-sys",
  "--allow-ffi",
  "--allow-write",
];

const EXIT_USAGE = 2;

interface RunResult {
  code: number;
  stdout: Uint8Array;
  stderr: string;
}

async function render(spec: unknown, ...args: string[]): Promise<RunResult> {
  const stdin = typeof spec === "string" ? spec : JSON.stringify(spec);
  const command = new Deno.Command(Deno.execPath(), {
    args: ["run", ...PERMISSIONS, SCRIPT, ...args],
    stdin: "piped",
    stdout: "piped",
    stderr: "piped",
  });
  const child = command.spawn();
  const writer = child.stdin.getWriter();
  await writer.write(new TextEncoder().encode(stdin));
  await writer.close();
  const { code, stdout, stderr } = await child.output();
  return { code, stdout, stderr: new TextDecoder().decode(stderr) };
}

function asText(bytes: Uint8Array): string {
  return new TextDecoder().decode(bytes);
}

function barSpec(rowCount = 2): Record<string, unknown> {
  const rows = Array.from({ length: rowCount }, (_, i) => ({
    entity: `file_${String(i).padStart(3, "0")}.go`,
    n_revs: rowCount - i,
  }));
  return {
    data: { values: rows },
    semantic_types: { entity: "Name", n_revs: "Count" },
    chart_spec: {
      chartType: "Bar Chart",
      encodings: { x: { field: "entity" }, y: { field: "n_revs" } },
    },
  };
}

// The recorded specs flint_spec.py emits for the six override envelopes in
// flint_spec_test.py (plan 3.4): the fixtures this renderer must accept.
const OVERRIDE_SPECS: Record<string, Record<string, unknown>> = {
  "code-age": {
    data: {
      values: [
        { entity: "a.go", age_months: 14 },
        { entity: "b.go", age_months: 2 },
        { entity: "c.go", age_months: 2 },
      ],
    },
    semantic_types: {
      entity: "Name",
      age_months: { semanticType: "Duration", unit: "months" },
    },
    chart_spec: {
      chartType: "Histogram",
      encodings: { x: { field: "age_months" } },
    },
  },
  summary: {
    data: {
      values: [
        { statistic: "number-of-commits", value: 12 },
        { statistic: "number-of-entities", value: 5 },
      ],
    },
    semantic_types: { statistic: "Category", value: "Count" },
    chart_spec: {
      chartType: "KPI Card",
      encodings: { metric: { field: "statistic" }, value: { field: "value" } },
    },
  },
  authors: {
    data: {
      values: [
        { entity: "a.go", n_authors: 3, n_revs: 20 },
        { entity: "b.go", n_authors: 1, n_revs: 4 },
      ],
    },
    semantic_types: { entity: "Name", n_authors: "Count", n_revs: "Count" },
    chart_spec: {
      chartType: "Scatter Plot",
      encodings: { x: { field: "n_revs" }, y: { field: "n_authors" } },
    },
  },
  coupling: {
    data: {
      values: [
        { entity: "a.go", coupled: "b.go", degree: 78, average_revs: 44 },
        { entity: "a.go", coupled: "c.go", degree: 62, average_revs: 45 },
      ],
    },
    semantic_types: {
      entity: "Name",
      coupled: "Name",
      degree: "Percentage",
      average_revs: "Count",
    },
    chart_spec: {
      chartType: "Network Graph",
      encodings: {
        x: { field: "entity" },
        y: { field: "coupled" },
        size: { field: "degree" },
      },
    },
  },
  communication: {
    data: {
      values: [
        { author: "alice", peer: "bob", shared: 8, average: 12, strength: 66 },
      ],
    },
    semantic_types: {
      author: "Name",
      peer: "Name",
      shared: "Count",
      average: "Count",
      strength: "Percentage",
    },
    chart_spec: {
      chartType: "Network Graph",
      encodings: {
        x: { field: "author" },
        y: { field: "peer" },
        size: { field: "strength" },
      },
    },
  },
  "absolute-churn": {
    data: {
      values: [
        { date: "2026-01-02", added: 120, deleted: 30, commits: 4 },
        { date: "2026-01-03", added: 20, deleted: 90, commits: 2 },
      ],
    },
    semantic_types: {
      date: "Date",
      added: { semanticType: "Quantity", unit: "lines" },
      deleted: { semanticType: "Quantity", unit: "lines" },
      commits: "Count",
    },
    chart_spec: {
      chartType: "Stacked Bar Chart",
      encodings: { x: { field: "date" }, y: ["added", "deleted"] },
    },
  },
};

Deno.test("renders a vegalite spec to SVG on stdout", async () => {
  const { code, stdout, stderr } = await render(barSpec());
  assertEquals(code, 0, stderr);
  const svg = asText(stdout);
  assert(svg.startsWith("<svg"), `not SVG: ${svg.slice(0, 60)}`);
  assert(stderr.includes("vegalite"), stderr);
});

Deno.test("rejects a channel the template does not declare", async () => {
  const spec = barSpec();
  (spec.chart_spec as { encodings: Record<string, unknown> }).encodings.size = {
    field: "n_revs",
  };
  const { code, stderr } = await render(spec);
  assertEquals(code, EXIT_USAGE, stderr);
  assert(stderr.includes("size"), stderr);
  assert(stderr.includes("Bar Chart"), stderr);
});

Deno.test("rejects an unknown chart type naming it", async () => {
  const spec = barSpec();
  (spec.chart_spec as { chartType: string }).chartType = "Nope Chart";
  const { code, stderr } = await render(spec);
  assertEquals(code, EXIT_USAGE, stderr);
  assert(stderr.includes("Nope Chart"), stderr);
});

Deno.test("re-emits Flint truncation warnings as stderr JSON lines", async () => {
  const { code, stderr } = await render(barSpec(300));
  assertEquals(code, 0, stderr);
  const warningLines = stderr
    .split("\n")
    .filter((line) => line.startsWith("{"))
    .map((line) => JSON.parse(line));
  assertEquals(warningLines.length, 1, stderr);
  assertEquals(warningLines[0].code, "overflow");
  assertEquals(warningLines[0].severity, "warning");
  assert(warningLines[0].message.includes("omitted"), stderr);
});

Deno.test("--format png emits PNG bytes", async () => {
  const { code, stdout, stderr } = await render(barSpec(), "--format", "png");
  assertEquals(code, 0, stderr);
  assertEquals(
    Array.from(stdout.slice(0, 4)),
    [0x89, 0x50, 0x4e, 0x47],
    stderr,
  );
  assert(stderr.includes("png"), stderr);
});

Deno.test("-o writes the artifact to a file, stdout stays empty", async () => {
  const out = await Deno.makeTempFile({ suffix: ".svg" });
  try {
    const { code, stdout, stderr } = await render(barSpec(), "-o", out);
    assertEquals(code, 0, stderr);
    assertEquals(stdout.length, 0, asText(stdout));
    assert((await Deno.readTextFile(out)).startsWith("<svg"), stderr);
  } finally {
    await Deno.remove(out);
  }
});

Deno.test("--scale enlarges the PNG raster", async () => {
  const one = await render(barSpec(), "--format", "png");
  const two = await render(barSpec(), "--format", "png", "--scale", "2");
  assertEquals(one.code, 0, one.stderr);
  assertEquals(two.code, 0, two.stderr);
  assert(
    two.stdout.length > one.stdout.length,
    `${one.stdout.length} vs ${two.stdout.length}`,
  );
});

Deno.test("rejects non-JSON stdin with exit 2", async () => {
  const { code, stderr } = await render("not json");
  assertEquals(code, EXIT_USAGE, stderr);
  assert(stderr.includes("JSON"), stderr);
});

Deno.test("rejects a payload without a chart_spec", async () => {
  const { code, stderr } = await render({ rows: [{ entity: "a.go" }] });
  assertEquals(code, EXIT_USAGE, stderr);
  assert(stderr.includes("chart_spec"), stderr);
});

Deno.test("infers echarts for the ECharts-only Network Graph", async () => {
  const { code, stdout, stderr } = await render(OVERRIDE_SPECS["coupling"]);
  assertEquals(code, 0, stderr);
  assert(asText(stdout).startsWith("<svg"), stderr);
  assert(stderr.includes("echarts"), stderr);
});

Deno.test("Network Graph SVG is byte-stable across runs", async () => {
  const first = await render(OVERRIDE_SPECS["coupling"]);
  const second = await render(OVERRIDE_SPECS["coupling"]);
  assertEquals(first.code, 0, first.stderr);
  assertEquals(asText(first.stdout), asText(second.stdout));
});

Deno.test("renders every override fixture from flint_spec.py", async () => {
  for (const [analysis, spec] of Object.entries(OVERRIDE_SPECS)) {
    const { code, stdout, stderr } = await render(spec);
    assertEquals(code, 0, `${analysis}: ${stderr}`);
    assert(asText(stdout).startsWith("<svg"), `${analysis}: not SVG`);
  }
});

Deno.test("--backend echarts picks the cross-backend Heatmap over vegalite", async () => {
  const spec = {
    data: {
      values: [{ entity: "a.go", coupled: "b.go", degree: 78 }],
    },
    semantic_types: { entity: "Name", coupled: "Name", degree: "Percentage" },
    chart_spec: {
      chartType: "Heatmap",
      encodings: {
        x: { field: "entity" },
        y: { field: "coupled" },
        color: { field: "degree" },
      },
    },
  };
  const plain = await render(spec);
  assertEquals(plain.code, 0, plain.stderr);
  assert(plain.stderr.includes("vegalite"), plain.stderr);
  const forced = await render(spec, "--backend", "echarts");
  assertEquals(forced.code, 0, forced.stderr);
  assert(forced.stderr.includes("echarts"), forced.stderr);
});

Deno.test("--backend without the template is a usage error", async () => {
  // KPI Card is a vegalite-only template at flint-chart@0.4.0.
  const { code, stderr } = await render(
    OVERRIDE_SPECS["summary"],
    "--backend",
    "echarts",
  );
  assertEquals(code, EXIT_USAGE, stderr);
  assert(stderr.includes("KPI Card"), stderr);
});

Deno.test("a spec Flint rejects exits 2 with a message, no stack trace", async () => {
  const spec = barSpec();
  (spec.chart_spec as { encodings: Record<string, unknown> }).encodings.y = {
    field: "missing_column",
  };
  const { code, stderr } = await render(spec);
  assertEquals(code, EXIT_USAGE, stderr);
  assert(stderr.includes("missing_column"), stderr);
  assert(!stderr.includes("    at "), stderr);
});
