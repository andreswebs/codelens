---
id: cod-y20g
status: closed
deps: [cod-8h5b]
links: []
created: 2026-07-14T03:40:09Z
type: task
priority: 1
assignee: Andre Silva
parent: cod-hkgg
tags: [codelens, spec-001, phase-1]
---

# P1-2 gitlog: blank-line entry tokenizer

Split a git2 log stream into blank-line-separated entry chunks, streaming, honoring input encoding. Internal to the gitlog package.

New files: src/internal/gitlog/tokenize.go, tokenize_test.go.

Docs: plan.md (Phase 1), design cli-design.md section 5, port reference docs/research/code-maat.md sections 2 (data model), 3 (log format incl 3.4 stacked preludes, 3.5 parser notes). Skills: /golang /tdd.
Reference: research 3.3 (on-disk shape), 3.5 (stream chunk-by-chunk). Depends on model (P1-1).

## Design

Internal surface (unexported, tested via package-internal test or via Parse in P1-3):

- func tokenize(r io.Reader) iter.Seq2[[]string, error] // yields one entry as its slice of lines (blank-line separated), or use a scanner returning [][]string for simplicity
- Blank line (possibly whitespace-only) delimits entries; consecutive blanks do not produce empty entries; trailing content without a final blank still yields a last entry.
- Encoding: caller decodes; default UTF-8. If --input-encoding is set, wrap the reader in a decoder (golang.org/x/text/encoding) before tokenizing. Keep the decoder wiring in gitlog.Parse (P1-3); tokenize works on decoded runes/lines.

Prefer the simplest correct implementation: bufio.Scanner over lines, accumulate non-blank lines into the current entry, flush on blank. Guard against a pathologically long line (bufio.Scanner buffer) by raising the buffer or returning an input error.

TDD cases:

1. TestTokenize_SingleEntry: one entry (no trailing blank) -> 1 chunk with all lines.
2. TestTokenize_MultipleEntries: two entries separated by one blank -> 2 chunks.
3. TestTokenize_ConsecutiveBlanks: extra blank lines between entries -> still 2 chunks, no empty chunk.
4. TestTokenize_Empty: "" -> 0 chunks.
5. TestTokenize_TrailingBlank: entry followed by blank -> 1 chunk (no empty trailing chunk).

## Acceptance Criteria

- Entries correctly delimited by blank lines; no empty chunks; streaming (does not require whole file in one string beyond the reader).
- All 5 cases pass; make validate green.

## Notes

**2026-07-14T10:35:19Z**

Implemented streaming blank-line tokenizer in src/internal/gitlog/tokenize.go (new gitlog package). Signature: tokenize(io.Reader) iter.Seq2[[]string, error] - yields each entry as its slice of lines (terminators stripped), streaming line-by-line via bufio.Scanner. Blank/whitespace-only lines delimit; consecutive blanks collapse (no empty chunks); leading/trailing blanks produce no empty chunk; trailing entry without final blank still yielded. CRLF handled (ScanLines drops trailing CR). Read errors and over-long lines (>maxLineSize=1MiB, via bufio.Scanner.Buffer) surface as a trailing (nil,err) pair with no partial entry emitted; early break stops the iterator. Caller (P1-3 Parse) owns --input-encoding decoding before the reader reaches tokenize, per design. Package doc comment lives here since gitlog is new. 11 tests (5 required + leading-blanks, CRLF, long-line, early-stop, reader-error). make build green.
