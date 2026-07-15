package analysis

import (
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// communicationRows asserts the result is an ok communication envelope and
// returns its rows as the concrete row type so cases can index into them.
func communicationRows(t *testing.T, rows any) []communicationRow {
	t.Helper()
	got, ok := rows.([]communicationRow)
	if !ok {
		t.Fatalf("rows is %T, want []communicationRow", rows)
	}
	return got
}

func TestComm_PairStrength(t *testing.T) {
	// alice and bob are the only (distinct) authors of two entities, so each
	// self-pair frequency is 2 and their shared frequency is 2:
	// average = ceil(avg(2,2)) = 2, strength = int(100 * 2/2) = 100. Both
	// directions are emitted (alice->bob and bob->alice), sorted author desc.
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "bob"},
		{Entity: "b.txt", Rev: "r3", Author: "alice"},
		{Entity: "b.txt", Rev: "r4", Author: "bob"},
	}

	res, err := runCommunication(mods, Opts{})
	if err != nil {
		t.Fatalf("runCommunication returned error: %v", err)
	}

	rows := communicationRows(t, res)
	want := []communicationRow{
		{Author: "bob", Peer: "alice", Shared: 2, Average: 2, Strength: 100},
		{Author: "alice", Peer: "bob", Shared: 2, Average: 2, Strength: 100},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestComm_SelfPairsExcludedFromOutput(t *testing.T) {
	// Self-pairs feed the frequency counts but are never emitted as rows.
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "bob"},
	}

	res, err := runCommunication(mods, Opts{})
	if err != nil {
		t.Fatalf("runCommunication returned error: %v", err)
	}

	for _, r := range communicationRows(t, res) {
		if r.Author == r.Peer {
			t.Errorf("self-pair row emitted: %+v", r)
		}
	}
}

func TestComm_SortStrengthDesc(t *testing.T) {
	// alice co-works with bob on two entities and with carol on one, and touches
	// a fourth entity alone. Touched-entity counts: alice=4, bob=2, carol=1.
	// alice<->bob: shared=2, average=ceil(avg(4,2))=3, strength=int(100*2/3)=66.
	// alice<->carol: shared=1, average=ceil(avg(4,1))=3, strength=int(100*1/3)=33.
	// Rows sort by strength desc, then author desc.
	mods := []model.Modification{
		{Entity: "a.txt", Rev: "r1", Author: "alice"},
		{Entity: "a.txt", Rev: "r2", Author: "bob"},
		{Entity: "b.txt", Rev: "r3", Author: "alice"},
		{Entity: "b.txt", Rev: "r4", Author: "bob"},
		{Entity: "c.txt", Rev: "r5", Author: "alice"},
		{Entity: "c.txt", Rev: "r6", Author: "carol"},
		{Entity: "d.txt", Rev: "r7", Author: "alice"},
	}

	res, err := runCommunication(mods, Opts{})
	if err != nil {
		t.Fatalf("runCommunication returned error: %v", err)
	}

	rows := communicationRows(t, res)
	want := []communicationRow{
		{Author: "bob", Peer: "alice", Shared: 2, Average: 3, Strength: 66},
		{Author: "alice", Peer: "bob", Shared: 2, Average: 3, Strength: 66},
		{Author: "carol", Peer: "alice", Shared: 1, Average: 3, Strength: 33},
		{Author: "alice", Peer: "carol", Shared: 1, Average: 3, Strength: 33},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %+v, want %+v", rows, want)
	}
}

func TestComm_Empty(t *testing.T) {
	res, err := runCommunication(nil, Opts{})
	if err != nil {
		t.Fatalf("runCommunication returned error: %v", err)
	}
	if rows := communicationRows(t, res); len(rows) != 0 {
		t.Errorf("rows = %+v, want empty", rows)
	}
}

func TestComm_DescriptorRegistered(t *testing.T) {
	d := communicationDescriptor()
	if d.Name != "communication" {
		t.Errorf("Name = %q, want %q", d.Name, "communication")
	}
	if len(d.Aliases) != 0 {
		t.Errorf("Aliases = %v, want none", d.Aliases)
	}
	wantCols := []string{"author", "peer", "shared", "average", "strength"}
	if len(d.RowSchema) != len(wantCols) {
		t.Fatalf("RowSchema has %d columns, want %d", len(d.RowSchema), len(wantCols))
	}
	for i, name := range wantCols {
		if d.RowSchema[i].Name != name {
			t.Errorf("RowSchema[%d].Name = %q, want %q", i, d.RowSchema[i].Name, name)
		}
	}
	if d.Run == nil {
		t.Error("Run is nil")
	}
	if got, ok := Lookup("communication"); !ok || got.Name != "communication" {
		t.Errorf("Lookup(communication) = %+v, %v; want registered descriptor", got, ok)
	}
}
