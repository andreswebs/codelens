package gitlog

import (
	"errors"
	"strings"
	"testing"
)

// collect drains tokenize into a slice of entries, failing the test on the
// first error the iterator yields.
func collect(t *testing.T, in string) [][]string {
	t.Helper()

	var got [][]string
	for entry, err := range tokenize(strings.NewReader(in)) {
		if err != nil {
			t.Fatalf("tokenize yielded error: %v", err)
		}
		got = append(got, entry)
	}
	return got
}

func TestTokenize_SingleEntry(t *testing.T) {
	in := "--586b4eb--2015-06-15--Adam Tornhill--Fix the parser\n" +
		"35\t0\tsrc/code_maat/parsers/git2.clj\n" +
		"-\t-\tdocs/logo.png"

	got := collect(t, in)

	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	want := []string{
		"--586b4eb--2015-06-15--Adam Tornhill--Fix the parser",
		"35\t0\tsrc/code_maat/parsers/git2.clj",
		"-\t-\tdocs/logo.png",
	}
	assertEntry(t, got[0], want)
}

func TestTokenize_MultipleEntries(t *testing.T) {
	in := "--aaa--2015-01-01--Alice--first\n" +
		"1\t0\ta.go\n" +
		"\n" +
		"--bbb--2015-01-02--Bob--second\n" +
		"2\t1\tb.go"

	got := collect(t, in)

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	assertEntry(t, got[0], []string{"--aaa--2015-01-01--Alice--first", "1\t0\ta.go"})
	assertEntry(t, got[1], []string{"--bbb--2015-01-02--Bob--second", "2\t1\tb.go"})
}

func TestTokenize_ConsecutiveBlanks(t *testing.T) {
	in := "--aaa--2015-01-01--Alice--first\n" +
		"1\t0\ta.go\n" +
		"\n" +
		"\n" +
		"   \n" + // whitespace-only line also counts as blank
		"--bbb--2015-01-02--Bob--second\n" +
		"2\t1\tb.go"

	got := collect(t, in)

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2 (no empty chunk from extra blanks)", len(got))
	}
	assertEntry(t, got[0], []string{"--aaa--2015-01-01--Alice--first", "1\t0\ta.go"})
	assertEntry(t, got[1], []string{"--bbb--2015-01-02--Bob--second", "2\t1\tb.go"})
}

func TestTokenize_Empty(t *testing.T) {
	got := collect(t, "")

	if len(got) != 0 {
		t.Fatalf("got %d entries, want 0", len(got))
	}
}

func TestTokenize_TrailingBlank(t *testing.T) {
	in := "--aaa--2015-01-01--Alice--first\n" +
		"1\t0\ta.go\n" +
		"\n"

	got := collect(t, in)

	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 (no empty trailing chunk)", len(got))
	}
	assertEntry(t, got[0], []string{"--aaa--2015-01-01--Alice--first", "1\t0\ta.go"})
}

// TestTokenize_LeadingBlanks documents that blank lines before the first entry
// do not produce an empty leading chunk.
func TestTokenize_LeadingBlanks(t *testing.T) {
	in := "\n\n--aaa--2015-01-01--Alice--first\n1\t0\ta.go"

	got := collect(t, in)

	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	assertEntry(t, got[0], []string{"--aaa--2015-01-01--Alice--first", "1\t0\ta.go"})
}

// TestTokenize_CRLF documents that Windows line endings are handled: the
// trailing carriage return is stripped and CRLF blank lines still delimit.
func TestTokenize_CRLF(t *testing.T) {
	in := "--aaa--2015-01-01--Alice--first\r\n" +
		"1\t0\ta.go\r\n" +
		"\r\n" +
		"--bbb--2015-01-02--Bob--second\r\n" +
		"2\t1\tb.go\r\n"

	got := collect(t, in)

	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	assertEntry(t, got[0], []string{"--aaa--2015-01-01--Alice--first", "1\t0\ta.go"})
	assertEntry(t, got[1], []string{"--bbb--2015-01-02--Bob--second", "2\t1\tb.go"})
}

// TestTokenize_LongLineError documents that a line exceeding the scan buffer
// surfaces as an error through the iterator rather than a silent truncation.
func TestTokenize_LongLineError(t *testing.T) {
	in := strings.Repeat("x", maxLineSize+1)

	var gotErr error
	var entries int
	for entry, err := range tokenize(strings.NewReader(in)) {
		if err != nil {
			gotErr = err
			continue
		}
		_ = entry
		entries++
	}

	if gotErr == nil {
		t.Fatal("expected an error for an over-long line, got nil")
	}
	if entries != 0 {
		t.Fatalf("got %d entries, want 0 when the line overflows", entries)
	}
}

// TestTokenize_EarlyStop documents that breaking out of the range stops the
// iterator and does not panic.
func TestTokenize_EarlyStop(t *testing.T) {
	in := "--aaa--2015-01-01--Alice--first\n\n--bbb--2015-01-02--Bob--second"

	count := 0
	for entry, err := range tokenize(strings.NewReader(in)) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = entry
		count++
		break
	}

	if count != 1 {
		t.Fatalf("consumed %d entries, want 1 before break", count)
	}
}

// TestTokenize_ErrorFromReader documents that a read failure is surfaced.
func TestTokenize_ErrorFromReader(t *testing.T) {
	wantErr := errors.New("boom")

	var gotErr error
	for _, err := range tokenize(errReader{err: wantErr}) {
		if err != nil {
			gotErr = err
		}
	}

	if !errors.Is(gotErr, wantErr) {
		t.Fatalf("gotErr = %v, want %v", gotErr, wantErr)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func assertEntry(t *testing.T, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("entry has %d lines, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}
