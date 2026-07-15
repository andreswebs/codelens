package analysis

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
	"github.com/andreswebs/codelens/internal/terr"
)

// assertCoded asserts err is a terr.Coded carrying the given code and exit code.
// The invalid_expression cases attach details via Wrap/WithDetails, which return
// a copy, so identity comparison (errors.Is) does not apply; the code is the
// stable contract.
func assertCoded(t *testing.T, err error, code string, exit int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var coded terr.Coded
	if !errors.As(err, &coded) {
		t.Fatalf("error %v is not a terr.Coded", err)
	}
	if coded.Code() != code {
		t.Errorf("code = %q, want %q", coded.Code(), code)
	}
	if coded.ExitCode() != exit {
		t.Errorf("exit = %d, want %d", coded.ExitCode(), exit)
	}
}

// msgMod is a terse constructor for the Modification fields the messages
// analysis reads: entity, revision, and the commit subject to match against.
func msgMod(entity, rev, message string) model.Modification {
	return model.Modification{
		Entity:  entity,
		Rev:     rev,
		Date:    "2024-01-01",
		Author:  "x",
		Message: message,
	}
}

// messagesRows asserts the result is an ok messages envelope and returns its
// rows as the concrete row type so cases can index into them.
func messagesRows(t *testing.T, rows any) []messagesRow {
	t.Helper()
	got, ok := rows.([]messagesRow)
	if !ok {
		t.Fatalf("rows is %T, want []messagesRow", rows)
	}
	return got
}

func TestMessages_CountsMatchesPerEntity(t *testing.T) {
	// Two entities carry commits mentioning "bug"; one carries only unrelated
	// subjects and must be excluded from the result.
	mods := []model.Modification{
		msgMod("A", "r1", "fix bug in parser"),
		msgMod("A", "r2", "another bug found"),
		msgMod("B", "r3", "bug: crash on start"),
		msgMod("C", "r4", "add feature"),
		msgMod("C", "r5", "refactor"),
	}

	res, err := runMessages(mods, Opts{Expression: "bug"})
	if err != nil {
		t.Fatalf("runMessages returned error: %v", err)
	}

	rows := messagesRows(t, res)
	want := []messagesRow{
		{Entity: "A", Matches: 2},
		{Entity: "B", Matches: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestMessages_MissingExpression(t *testing.T) {
	mods := []model.Modification{msgMod("A", "r1", "fix bug")}

	_, err := runMessages(mods, Opts{Expression: ""})
	assertCoded(t, err, "invalid_expression", 2)
}

func TestMessages_NoMessagesLog(t *testing.T) {
	// A stock 3-field log has every message defaulted to "-", so a messages
	// analysis has nothing to match: it is an input error (exit 3).
	mods := []model.Modification{
		msgMod("A", "r1", "-"),
		msgMod("B", "r2", "-"),
	}

	_, err := runMessages(mods, Opts{Expression: "bug"})
	if !errors.Is(err, ErrMissingMessages) {
		t.Fatalf("runMessages error = %v, want ErrMissingMessages", err)
	}
	if got := ErrMissingMessages.ExitCode(); got != 3 {
		t.Errorf("ErrMissingMessages exit code = %d, want 3", got)
	}
}

func TestMessages_InvalidExpression(t *testing.T) {
	mods := []model.Modification{msgMod("A", "r1", "fix bug")}

	_, err := runMessages(mods, Opts{Expression: "("})
	assertCoded(t, err, "invalid_expression", 2)
}

func TestMessages_OversizedExpression(t *testing.T) {
	mods := []model.Modification{msgMod("A", "r1", "fix bug")}
	huge := strings.Repeat("a", maxExpressionLen+1)

	_, err := runMessages(mods, Opts{Expression: huge})
	assertCoded(t, err, "invalid_expression", 2)
}

func TestMessages_SortMatchesDesc(t *testing.T) {
	// Higher match counts come first; entities with equal counts break the tie
	// on entity descending, matching code-maat's [matches, entity] :desc order.
	mods := []model.Modification{
		msgMod("A", "r1", "bug one"),
		msgMod("B", "r2", "bug two"),
		msgMod("B", "r3", "bug three"),
		msgMod("C", "r4", "bug four"),
	}

	res, err := runMessages(mods, Opts{Expression: "bug"})
	if err != nil {
		t.Fatalf("runMessages returned error: %v", err)
	}

	rows := messagesRows(t, res)
	want := []messagesRow{
		{Entity: "B", Matches: 2},
		{Entity: "C", Matches: 1},
		{Entity: "A", Matches: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestMessages_EmptyLog(t *testing.T) {
	// No modifications at all is not a missing-messages error; it yields an
	// empty-but-valid result (the empty_log case is handled by the parser).
	res, err := runMessages(nil, Opts{Expression: "bug"})
	if err != nil {
		t.Fatalf("runMessages returned error: %v", err)
	}
	rows := messagesRows(t, res)
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestMessages_DescriptorRegistered(t *testing.T) {
	d := messagesDescriptor()
	if d.Name != "messages" {
		t.Errorf("Name = %q, want %q", d.Name, "messages")
	}
	if len(d.Flags) != 1 || d.Flags[0].Name != "expression" || !d.Flags[0].Required {
		t.Errorf("Flags = %+v, want one required expression flag", d.Flags)
	}
	wantCols := []string{"entity", "matches"}
	if len(d.RowSchema) != len(wantCols) {
		t.Fatalf("RowSchema has %d columns, want %d", len(d.RowSchema), len(wantCols))
	}
	for i, name := range wantCols {
		if d.RowSchema[i].Name != name {
			t.Errorf("RowSchema[%d].Name = %q, want %q", i, d.RowSchema[i].Name, name)
		}
	}
	wantCodes := []string{"empty_log", "missing_messages", "invalid_expression"}
	if !reflect.DeepEqual(d.ErrorCodes, wantCodes) {
		t.Errorf("ErrorCodes = %v, want %v", d.ErrorCodes, wantCodes)
	}
	if !reflect.DeepEqual(d.ExitCodes, []int{0, 2, 3, 1}) {
		t.Errorf("ExitCodes = %v, want [0 2 3 1]", d.ExitCodes)
	}
}
