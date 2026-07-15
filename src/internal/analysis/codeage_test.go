package analysis

import (
	"reflect"
	"testing"
	"time"

	"github.com/andreswebs/codelens/internal/model"
)

// ageMod is a terse constructor for the Modification fields the code-age
// analysis reads: entity, revision, and the commit date. Loc/author fields are
// irrelevant to age and left zero.
func ageMod(entity, rev, date string) model.Modification {
	return model.Modification{Entity: entity, Rev: rev, Date: date}
}

// codeAgeRows asserts the result is an ok code-age envelope and returns its rows
// as the concrete row type so cases can index into them.
func codeAgeRows(t *testing.T, rows any) []codeAgeRow {
	t.Helper()
	got, ok := rows.([]codeAgeRow)
	if !ok {
		t.Fatalf("rows is %T, want []codeAgeRow", rows)
	}
	return got
}

func TestCodeAge_MonthsSinceLatest(t *testing.T) {
	// The latest change strictly before now is 2020-03-10; from there to
	// 2020-06-15 spans three whole calendar months (day 15 >= day 10, so the
	// final month is complete).
	mods := []model.Modification{
		ageMod("A", "r1", "2020-01-15"),
		ageMod("A", "r2", "2020-03-10"),
	}

	res, err := runCodeAge(mods, Opts{TimeNow: "2020-06-15"})
	if err != nil {
		t.Fatalf("runCodeAge returned error: %v", err)
	}

	rows := codeAgeRows(t, res)
	want := []codeAgeRow{{Entity: "A", AgeMonths: 3}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestCodeAge_PartialMonthNotCounted(t *testing.T) {
	// From 2020-01-15 to 2020-02-10 is not a full month: day 10 < day 15, so
	// the difference is zero whole months.
	mods := []model.Modification{ageMod("A", "r1", "2020-01-15")}

	res, err := runCodeAge(mods, Opts{TimeNow: "2020-02-10"})
	if err != nil {
		t.Fatalf("runCodeAge returned error: %v", err)
	}

	rows := codeAgeRows(t, res)
	want := []codeAgeRow{{Entity: "A", AgeMonths: 0}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestCodeAge_IgnoresFutureChanges(t *testing.T) {
	// Only changes strictly before now count: the change on now (2020-06-15)
	// and the one after it are excluded, so the latest considered date is
	// 2020-01-10 -> five whole months.
	mods := []model.Modification{
		ageMod("A", "r1", "2020-01-10"),
		ageMod("A", "r2", "2020-06-15"),
		ageMod("A", "r3", "2020-07-01"),
	}

	res, err := runCodeAge(mods, Opts{TimeNow: "2020-06-15"})
	if err != nil {
		t.Fatalf("runCodeAge returned error: %v", err)
	}

	rows := codeAgeRows(t, res)
	want := []codeAgeRow{{Entity: "A", AgeMonths: 5}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestCodeAge_EntityWithoutPastChangesSkipped(t *testing.T) {
	// past has a change before now; future's only change is on now itself, so
	// it is dropped entirely rather than emitted with a zero or negative age.
	mods := []model.Modification{
		ageMod("past", "r1", "2020-01-10"),
		ageMod("future", "r2", "2020-06-15"),
	}

	res, err := runCodeAge(mods, Opts{TimeNow: "2020-06-15"})
	if err != nil {
		t.Fatalf("runCodeAge returned error: %v", err)
	}

	rows := codeAgeRows(t, res)
	want := []codeAgeRow{{Entity: "past", AgeMonths: 5}}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestCodeAge_SortAscTieByEntity(t *testing.T) {
	// young (1 month) sorts before the two 3-month entities; among equal ages,
	// entity name breaks the tie ascending (alpha before beta).
	mods := []model.Modification{
		ageMod("beta", "r1", "2020-03-10"),
		ageMod("young", "r2", "2020-05-10"),
		ageMod("alpha", "r3", "2020-03-10"),
	}

	res, err := runCodeAge(mods, Opts{TimeNow: "2020-06-15"})
	if err != nil {
		t.Fatalf("runCodeAge returned error: %v", err)
	}

	rows := codeAgeRows(t, res)
	want := []codeAgeRow{
		{Entity: "young", AgeMonths: 1},
		{Entity: "alpha", AgeMonths: 3},
		{Entity: "beta", AgeMonths: 3},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestCodeAge_DefaultNowIsToday(t *testing.T) {
	// An empty --time-now resolves to today's UTC calendar date.
	got, err := resolveNow("")
	if err != nil {
		t.Fatalf("resolveNow(\"\") returned error: %v", err)
	}
	now := time.Now().UTC()
	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("resolveNow(\"\") = %v, want %v", got, want)
	}
}

func TestCodeAge_BadTimeNow(t *testing.T) {
	mods := []model.Modification{ageMod("A", "r1", "2020-01-10")}

	_, err := runCodeAge(mods, Opts{TimeNow: "not-a-date"})
	assertCoded(t, err, "invalid_time_now", 2)
}
