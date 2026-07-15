package gitlog

import (
	"errors"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

func TestParse_SingleEntry(t *testing.T) {
	in := "--990442e--2013-08-29--Adam Petersen\n" +
		"1\t0\tsrc/foo.clj\n" +
		"2\t4\tsrc/bar.clj"

	got := parseOK(t, in)

	want := []model.Modification{
		{Entity: "src/foo.clj", Rev: "990442e", Date: "2013-08-29", Author: "Adam Petersen", Message: "-", LocAdded: 1, LocDeleted: 0, HasLoc: true},
		{Entity: "src/bar.clj", Rev: "990442e", Date: "2013-08-29", Author: "Adam Petersen", Message: "-", LocAdded: 2, LocDeleted: 4, HasLoc: true},
	}
	assertMods(t, got, want)
}

func TestParse_BinaryEntry(t *testing.T) {
	in := "--586b4eb--2015-06-15--Adam Tornhill\n" +
		"-\t-\tproject.bin\n" +
		"2\t40\tsrc/foo.clj"

	got := parseOK(t, in)

	want := []model.Modification{
		{Entity: "project.bin", Rev: "586b4eb", Date: "2015-06-15", Author: "Adam Tornhill", Message: "-", Binary: true, HasLoc: true},
		{Entity: "src/foo.clj", Rev: "586b4eb", Date: "2015-06-15", Author: "Adam Tornhill", Message: "-", LocAdded: 2, LocDeleted: 40, HasLoc: true},
	}
	assertMods(t, got, want)
}

func TestParse_MultipleEntries(t *testing.T) {
	in := "--b777738--2015-01-01--Alice\n" +
		"1\t0\ta.go\n" +
		"3\t2\tb.go\n" +
		"\n" +
		"--a527b79--2015-01-02--Bob\n" +
		"1\t0\tc.go\n" +
		"1\t0\td.go\n" +
		"1\t0\te.go\n" +
		"1\t0\tf.go"

	got := parseOK(t, in)

	if len(got) != 6 {
		t.Fatalf("got %d modifications, want 6: %+v", len(got), got)
	}
	wantRevs := []string{"b777738", "b777738", "a527b79", "a527b79", "a527b79", "a527b79"}
	for i, rev := range wantRevs {
		if got[i].Rev != rev {
			t.Errorf("modification %d rev = %q, want %q", i, got[i].Rev, rev)
		}
	}
}

// TestParse_EmptyLog documents that empty input is an empty-log error, not a
// silent empty result: there is nothing to analyze.
func TestParse_EmptyLog(t *testing.T) {
	_, err := ParseString("", model.Options{})

	assertCoded(t, err, ErrEmptyLog, 3)
}

// TestParse_WhitespaceOnlyLog documents that a log with only blank lines yields
// no entries and is likewise treated as empty.
func TestParse_WhitespaceOnlyLog(t *testing.T) {
	_, err := ParseString("   \n\t\n \n", model.Options{})

	assertCoded(t, err, ErrEmptyLog, 3)
}

// TestParse_MalformedNumstat documents that a non-numstat line where a numstat
// is expected is a coded parse error naming the entry and offending line.
func TestParse_MalformedNumstat(t *testing.T) {
	in := "--990442e--2013-08-29--Adam Petersen\n" +
		"not-a-numstat"

	_, err := ParseString(in, model.Options{})

	assertCoded(t, err, ErrParse, 3)
	assertDetails(t, err, 1, "not-a-numstat")
}

// TestParse_BadDate documents that a non-date in the prelude's date field is a
// coded parse error carrying the offending prelude line.
func TestParse_BadDate(t *testing.T) {
	in := "--990442e--not-a-date--Adam Petersen\n" +
		"1\t0\tsrc/foo.clj"

	_, err := ParseString(in, model.Options{})

	assertCoded(t, err, ErrParse, 3)
	assertDetails(t, err, 1, "--990442e--not-a-date--Adam Petersen")
}

// TestParse_ControlChar documents that a NUL (or other disallowed control
// character) anywhere in the input is rejected as an input error.
func TestParse_ControlChar(t *testing.T) {
	in := "--990442e--2013-08-29--Adam Petersen\n" +
		"1\t0\tsrc/\x00foo.clj"

	_, err := ParseString(in, model.Options{})

	assertCoded(t, err, ErrControlChar, 3)
}

// TestParse_EmptyMerge documents that an entry with a prelude but no numstat
// lines (e.g. an empty merge) contributes no records.
func TestParse_EmptyMerge(t *testing.T) {
	got := parseOK(t, "--586b4eb--2015-06-15--Adam Tornhill")

	if len(got) != 0 {
		t.Fatalf("got %d modifications, want 0: %+v", len(got), got)
	}
}

// TestParse_PullRequestPreludes documents that stacked prelude lines collapse to
// the last one, which supplies rev/date/author.
func TestParse_PullRequestPreludes(t *testing.T) {
	in := "--aaaaaaa--2015-06-14--Mr X\n" +
		"--77c8751--2015-06-15--Mr Y\n" +
		"1\t0\ta.go\n" +
		"2\t0\tb.go"

	got := parseOK(t, in)

	if len(got) != 2 {
		t.Fatalf("got %d modifications, want 2: %+v", len(got), got)
	}
	for i, mod := range got {
		if mod.Rev != "77c8751" {
			t.Errorf("modification %d rev = %q, want %q (last prelude)", i, mod.Rev, "77c8751")
		}
		if mod.Author != "Mr Y" {
			t.Errorf("modification %d author = %q, want %q (last prelude)", i, mod.Author, "Mr Y")
		}
	}
}

// TestParse_Subject documents that the fourth prelude field is captured as the
// commit message.
func TestParse_Subject(t *testing.T) {
	got := parseOK(t, "--abc--2024-01-02--Jane Doe--Fix parser bug\n1\t0\ta.go")

	if len(got) != 1 {
		t.Fatalf("got %d modifications, want 1: %+v", len(got), got)
	}
	if got[0].Message != "Fix parser bug" {
		t.Errorf("Message = %q, want %q", got[0].Message, "Fix parser bug")
	}
}

// TestParse_SubjectWithDashes documents that a subject containing "--" is
// rejoined intact rather than truncated at the delimiter.
func TestParse_SubjectWithDashes(t *testing.T) {
	got := parseOK(t, "--def--2024-01-03--Jane--refactor: split a--b module\n1\t0\ta.go")

	if len(got) != 1 {
		t.Fatalf("got %d modifications, want 1: %+v", len(got), got)
	}
	if got[0].Message != "refactor: split a--b module" {
		t.Errorf("Message = %q, want %q", got[0].Message, "refactor: split a--b module")
	}
}

// parseOK runs ParseString and fails the test on any error.
func parseOK(t *testing.T, in string) []model.Modification {
	t.Helper()

	got, err := ParseString(in, model.Options{})
	if err != nil {
		t.Fatalf("ParseString returned error: %v", err)
	}
	return got
}

// assertCoded fails unless err is a coded error reporting the same code and
// exit code as the wanted sentinel. Codes are compared rather than pointer
// identity because Wrap/WithDetails return copies of the sentinel.
func assertCoded(t *testing.T, err error, want *terr.Error, wantExit int) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error %v, got nil", want)
	}
	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error %v is not coded", err)
	}
	if coded.Code() != want.Code() {
		t.Errorf("code = %q, want %q", coded.Code(), want.Code())
	}
	if coded.ExitCode() != wantExit {
		t.Errorf("exit code = %d, want %d", coded.ExitCode(), wantExit)
	}
}

// assertDetails fails unless err carries structured parse details naming the
// expected entry index and offending line.
func assertDetails(t *testing.T, err error, wantEntry int, wantLine string) {
	t.Helper()

	var detailed terr.Detailed
	if !errors.As(err, &detailed) {
		t.Fatalf("error %v carries no details", err)
	}
	d, ok := detailed.ErrorDetails().(map[string]any)
	if !ok {
		t.Fatalf("details = %#v, want map[string]any", detailed.ErrorDetails())
	}
	if got := d["entry"]; got != wantEntry {
		t.Errorf("details entry = %v, want %d", got, wantEntry)
	}
	if got := d["line"]; got != wantLine {
		t.Errorf("details line = %q, want %q", got, wantLine)
	}
}

func assertMods(t *testing.T, got, want []model.Modification) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d modifications, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("modification %d =\n  %+v\nwant\n  %+v", i, got[i], want[i])
		}
	}
}
