# /// script
# requires-python = ">=3.12"
# dependencies = ["wordcloud", "matplotlib"]
# ///
# matplotlib types methods with **kwargs: Unknown and wordcloud ships no stubs, so
# both read as partially unknown under strict; those third-party rules are off here.
# pyright: reportUnknownMemberType=false, reportMissingTypeStubs=false
"""Commit word cloud from codelens `parse` output.

Named commit_cloud (not wordcloud) so the script does not shadow the `wordcloud`
PyPI package it imports. Consumes `codelens parse` JSON and tokenizes the
`message` column. parse emits one row per file per commit, so messages are
de-duplicated by `rev` before counting. Dominant words show where the team spends
time: domain terms are healthy; "bug", "crash", "revert", "bump" warrant a
drill-down.

Usage:
  codelens parse --format json | uv run scripts/commit_cloud.py -o cloud.svg
  uv run scripts/commit_cloud.py -i parse.json --extra-stopwords bump,wip -o cloud.svg
Exit codes: 0 ok; 2 usage; 3 no messages.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any, NoReturn, cast

TOKEN = re.compile(r"[A-Za-z][A-Za-z'+-]{1,}")


def die(msg: str, code: int) -> NoReturn:
    print(f"commit_cloud.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def load(path: str) -> list[dict[str, Any]]:
    text = sys.stdin.read() if path == "-" else Path(path).read_text(encoding="utf-8")
    doc: Any = json.loads(text)
    data = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    if not isinstance(data, list):
        die("no rows array", 2)
    return cast("list[dict[str, Any]]", data)


def main() -> None:
    ap = argparse.ArgumentParser(description="Commit-message word cloud.")
    ap.add_argument(
        "-i", "--input", default="-", help="codelens parse JSON, or '-' for stdin"
    )
    ap.add_argument(
        "--extra-stopwords", default="", help="comma-separated words to also drop"
    )
    ap.add_argument("-o", "--out", required=True)
    args = ap.parse_args()

    # De-duplicate messages by revision (parse repeats the message per changed file).
    by_rev: dict[str, str] = {}
    for r in load(args.input):
        msg = r.get("message")
        if msg:
            by_rev[r.get("rev", msg)] = msg
    if not by_rev:
        die("no commit messages in input (was the log built with the subject?)", 3)

    from wordcloud import STOPWORDS, WordCloud

    stop = set(STOPWORDS)
    stop.update(w.strip().lower() for w in args.extra_stopwords.split(",") if w.strip())

    freq: dict[str, int] = {}
    for msg in by_rev.values():
        for tok in TOKEN.findall(msg.lower()):
            if tok in stop or len(tok) < 3:
                continue
            freq[tok] = freq.get(tok, 0) + 1
    if not freq:
        die("no words survived filtering", 3)

    import matplotlib

    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    wc = WordCloud(
        width=1200,
        height=700,
        background_color="white",
        prefer_horizontal=0.9,
        collocations=False,
    ).generate_from_frequencies(freq)
    fig, ax = plt.subplots(figsize=(12, 7))
    ax.imshow(wc, interpolation="bilinear")
    ax.axis("off")
    fig.tight_layout(pad=0)
    fig.savefig(args.out)
    print(
        f"wrote {args.out} ({len(by_rev)} commits, {len(freq)} distinct words)",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
