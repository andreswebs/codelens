# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Condense a repo's codelens analysis JSON into a compact digest (markdown).

The digest is the grounding input for hand-written findings: it surfaces the
signal (top hotspots split code vs docs/config, coupling pairs, ownership
concentration, fragmentation, code-age range, churn shape, author pairs, commit
vocabulary) so a reader (or a findings-writing agent) never has to open the
multi-megabyte analysis JSON.

It reads the conventional per-analysis JSON files from a single directory (the
names the run produces; see references/reporting.md):

  summary.json revisions.json coupling.json main-dev.json fragmentation.json
  code-age.json abs-churn.json communication.json parse.json

A missing or empty file just omits its section. An optional git.log in the same
directory is used only to report the analysis window's first and last dates.

Input: a positional data directory. Output: -o FILE (default <dir>/digest.md).
Exit codes: 0 ok; 2 usage/bad input; 3 nothing to digest (no analysis files).
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter
from pathlib import Path
from typing import Any, NoReturn, cast

EXIT_USAGE = 2
EXIT_EMPTY = 3

# Files whose presence means there is something to digest. Absence of all of
# these is the empty case (exit 3), not a per-section omission.
ANALYSIS_FILES = (
    "summary.json",
    "revisions.json",
    "coupling.json",
    "main-dev.json",
    "fragmentation.json",
    "code-age.json",
    "abs-churn.json",
    "communication.json",
)

# Heuristic split of authored code from human-authored docs/config and generated
# reference data. This is a reading aid for the hotspot list, not an exclude: it
# only labels rows, it never drops them.
NONCODE = re.compile(
    r"(\.md$|\.mdx$|\.ya?ml$|\.json$|\.env|\.gitignore|\.github/|Dockerfile|"
    r"\.lock$|composer\.json|package\.json|\.txt$|\.toml$|\.ini$|\.neon$|"
    r"LICENSE|README|CHANGELOG|\.editorconfig|\.xml$|\.dist$|\.example$)",
    re.I,
)

# Common English + git/commit filler dropped from the commit-vocabulary tally.
STOP = set(
    "the a an and or to of in for on with by from at is be as it this that add "
    "added update updated fix fixed change changed remove removed new use using "
    "into out up down not no wip test tests merge branch pull request pr into "
    "feat feature chore refactor bump via but if then than can will".split()
)


def die(msg: str, code: int) -> NoReturn:
    print(f"digest.py: {msg}", file=sys.stderr)
    raise SystemExit(code)


def load_rows(path: Path) -> list[dict[str, Any]]:
    """Return the row list from a codelens JSON envelope ({"rows": [...]}) or a
    bare list. A missing, empty, or unparseable file yields an empty list, so a
    partial analysis run still digests what it has."""
    if not path.is_file() or path.stat().st_size == 0:
        return []
    try:
        doc: Any = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return []
    rows = cast("dict[str, Any]", doc).get("rows") if isinstance(doc, dict) else doc
    return cast("list[dict[str, Any]]", rows) if isinstance(rows, list) else []


def is_code(entity: str) -> bool:
    return NONCODE.search(entity) is None


def build_digest(d: Path) -> list[str]:
    """Build the digest lines for the analysis directory `d`."""
    out: list[str] = []
    w = out.append

    # --- summary ---
    summ: dict[str, Any] = {r["statistic"]: r["value"] for r in load_rows(d / "summary.json")}
    w("## summary")
    w(f"- commits: {summ.get('number-of-commits')}")
    w(f"- authors: {summ.get('number-of-authors')}")
    w(f"- entities: {summ.get('number-of-entities')}")
    w(f"- entity-changes: {summ.get('number-of-entities-changed')}")

    # --- window (from git.log first/last dates) ---
    gl = d / "git.log"
    if gl.is_file():
        text = gl.read_text(encoding="utf-8")
        dates: list[str] = re.findall(r"\b\d{4}-\d{2}-\d{2}\b", text[:2000] + text[-2000:])
        if dates:
            w(f"- window dates seen: {min(dates)} .. {max(dates)}")

    # --- hotspots (revisions), split code vs docs/config ---
    revs = load_rows(d / "revisions.json")
    code = [r for r in revs if is_code(str(r["entity"]))]
    noncode = [r for r in revs if not is_code(str(r["entity"]))]
    w("\n## hotspots (top code files by revisions)")
    for r in code[:12]:
        w(f"- {r['n_revs']:>4}  {r['entity']}")
    w("## top docs/config churn (context, not code hotspots)")
    for r in noncode[:6]:
        w(f"- {r['n_revs']:>4}  {r['entity']}")

    # --- coupling ---
    cp = load_rows(d / "coupling.json")
    w("\n## change coupling (degree = % shared commits)")
    if not cp:
        w("- (none above min-degree threshold; low temporal coupling or thin history)")
    for r in cp[:12]:
        w(f"- {r.get('degree'):>3}%  avg_revs={r.get('average_revs')}  {r['entity']}  <->  {r['coupled']}")

    # --- ownership ---
    code_md = [r for r in load_rows(d / "main-dev.json") if is_code(str(r["entity"]))]
    w("\n## ownership")
    if code_md:
        owned100 = sum(1 for r in code_md if float(r.get("ownership", 0)) >= 0.999)
        w(f"- code files with a single 100%-owner: {owned100}/{len(code_md)}")
        top = Counter(str(r["main_dev"]) for r in code_md).most_common(8)
        w("- main-developer by #code-files owned:")
        for author, n in top:
            w(f"  - {n:>4}  {author}")

    # --- fragmentation (code only) ---
    fr = [r for r in load_rows(d / "fragmentation.json") if is_code(str(r["entity"]))]
    fr.sort(key=lambda r: -float(r.get("fractal_value", 0)))
    w("\n## most fragmented code files (fractal value, higher = more split ownership)")
    for r in fr[:8]:
        w(f"- {float(r.get('fractal_value', 0)):.3f}  revs={r.get('total_revs')}  {r['entity']}")

    # --- code age ---
    age = [r for r in load_rows(d / "code-age.json") if is_code(str(r["entity"]))]
    if age:
        ages = sorted(int(r["age_months"]) for r in age)
        med = ages[len(ages) // 2]
        w("\n## code age (months since last change, code files)")
        w(f"- min={ages[0]} median={med} max={ages[-1]}")
        oldest = sorted(age, key=lambda r: -int(r["age_months"]))[:5]
        w("- oldest surviving code files:")
        for r in oldest:
            w(f"  - {r['age_months']:>3}mo  {r['entity']}")

    # --- churn ---
    ch = load_rows(d / "abs-churn.json")
    if ch:
        tot_add = sum(int(r.get("added", 0)) for r in ch)
        tot_del = sum(int(r.get("deleted", 0)) for r in ch)
        spike = max(ch, key=lambda r: int(r.get("added", 0)) + int(r.get("deleted", 0)))
        ratio = f"{tot_add / tot_del:.2f}" if tot_del else "inf"
        w("\n## churn")
        w(f"- periods={len(ch)} total_added={tot_add} total_deleted={tot_del} add/del_ratio={ratio}")
        w(f"- biggest period: {spike.get('date')} +{spike.get('added')}/-{spike.get('deleted')}")

    # --- communication ---
    comm = load_rows(d / "communication.json")
    w("\n## communication (author pairs; coordination, NOT performance)")
    authors = {str(r["author"]) for r in comm} | {str(r["peer"]) for r in comm}
    w(f"- authors in graph: {len(authors)}")
    for r in sorted(comm, key=lambda r: -int(r.get("strength", 0)))[:8]:
        w(f"- strength={r.get('strength'):>3} shared={r.get('shared')}  {r['author']} <-> {r['peer']}")

    # --- commit vocabulary ---
    words: Counter[str] = Counter()
    for r in load_rows(d / "parse.json"):
        msg = str(r.get("message") or "").lower()
        for tok in re.findall(r"[a-z][a-z0-9_-]{2,}", msg):
            if tok not in STOP:
                words[tok] += 1
    w("\n## commit vocabulary (heuristic, top terms)")
    w("- " + ", ".join(f"{word}({n})" for word, n in words.most_common(20)))

    return out


def main() -> None:
    ap = argparse.ArgumentParser(
        description="Condense a repo's codelens analysis JSON into a compact digest."
    )
    ap.add_argument("data_dir", help="directory of per-analysis codelens JSON files")
    ap.add_argument("-o", "--out", help="output file (default <data_dir>/digest.md)")
    args = ap.parse_args()

    d = Path(args.data_dir)
    if not d.is_dir():
        die(f"not a directory: {d}", EXIT_USAGE)
    if not any((d / name).is_file() for name in ANALYSIS_FILES):
        die(f"no analysis JSON found in {d}", EXIT_EMPTY)

    lines = build_digest(d)
    out = Path(args.out) if args.out else d / "digest.md"
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(f"wrote {out} ({len(lines)} lines)", file=sys.stderr)


if __name__ == "__main__":
    main()
