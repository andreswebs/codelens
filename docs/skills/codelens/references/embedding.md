# Embedding artifacts

Loaded from [SKILL.md](../SKILL.md) step 4. Each visualization produces a
canonical form; pick the artifact that fits the target.

- **Static** visualizations (churn, fractal, word cloud, complexity trend,
  summary) emit **SVG** (canonical) and **PNG** (fallback).
- **Interactive** visualizations (enclosure, coupling, network) emit an **HTML**
  file for live viewing and iframe embedding. They are not exported to static
  images; when a slide or PDF needs a picture, use a static visualization.

## Format to target

| Format           | HTML page                 | Slides           | PDF            |
| ---------------- | ------------------------- | ---------------- | -------------- |
| SVG              | inline `<svg>` or `<img>` | import as image  | vector, scales |
| PNG              | `<img>`                   | paste anywhere   | embed as image |
| Interactive HTML | `<iframe>`                | reveal.js iframe | not applicable |

## Mechanics

- **Inline SVG:** paste the `<svg>...</svg>` straight into HTML or Markdown; it
  scales and stays crisp in print. The everywhere format for static charts.
- **Iframe interactive HTML:** `<iframe src="hotspots.html">` on a web page, or a
  reveal.js `<section>` with an iframe, keeps zoom and tooltips.

## Interactive HTML self-containment

The interactive templates inline the data as a `<script>` JSON blob and load D3
from a CDN. The file therefore opens directly in a browser with no local server
(no `python -m http.server` CORS step), but needs network access to fetch D3.
