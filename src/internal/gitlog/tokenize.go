// Package gitlog parses the git2 log format (extended with the commit subject)
// into the flat model.Modification records every analysis consumes. The parser
// is split into a streaming tokenizer, which chunks the log into blank-line-
// separated entries, and an entry parser layered on top of it.
package gitlog

import (
	"bufio"
	"io"
	"iter"
	"strings"
)

// maxLineSize bounds a single log line so a pathological input cannot force
// unbounded buffering. Prelude subjects and numstat paths are far shorter; a
// line past this limit surfaces as an error rather than a silent truncation.
const maxLineSize = 1 << 20 // 1 MiB

// tokenize splits a git2 log stream into blank-line-separated entries, yielding
// each entry as its slice of lines with terminators stripped. It streams line
// by line rather than buffering the whole input.
//
// A blank or whitespace-only line delimits entries; consecutive blank lines
// collapse so no empty entry is produced, and a final entry without a trailing
// blank line is still yielded. Callers may stop early by breaking the range.
// If reading fails (including a line longer than maxLineSize), the iterator
// yields a single trailing (nil, err) pair and stops without emitting a partial
// entry. The caller is responsible for any character-encoding decoding before
// the reader reaches tokenize.
func tokenize(r io.Reader) iter.Seq2[[]string, error] {
	return func(yield func([]string, error) bool) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxLineSize)

		var entry []string
		for sc.Scan() {
			line := sc.Text()
			if hasControlChar(line) {
				yield(nil, ErrControlChar)
				return
			}
			if strings.TrimSpace(line) == "" {
				if len(entry) > 0 {
					if !yield(entry, nil) {
						return
					}
					entry = nil
				}
				continue
			}
			entry = append(entry, line)
		}

		if err := sc.Err(); err != nil {
			yield(nil, err)
			return
		}
		if len(entry) > 0 {
			yield(entry, nil)
		}
	}
}

// hasControlChar reports whether s contains a disallowed control character. Tab
// is permitted (numstat fields are tab-separated); the scanner has already
// stripped the line terminator, so newlines never reach here. Bytes at or above
// 0x80 are UTF-8 sequence bytes and are always allowed.
func hasControlChar(s string) bool {
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '\t' {
			continue
		}
		if b < 0x20 || b == 0x7f {
			return true
		}
	}
	return false
}
