package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// weakCouplingLog builds a git2+subject log where a.go and b.go co-change in
// `shared` commits and each also changes alone in `alone` further commits. With
// shared=5 and alone=12 each entity has 17 revisions, so the pair's degree is
// 5/17 = 29%, just under the default --min-coupling 30: a candidate pair that is
// nonetheless filtered out.
func weakCouplingLog(shared, alone int) string {
	var b strings.Builder
	n := 0
	entry := func(files ...string) {
		n++
		fmt.Fprintf(&b, "--%07x--2024-01-01--Alice--c%d\n", n, n)
		for _, f := range files {
			fmt.Fprintf(&b, "1\t0\t%s\n", f)
		}
		b.WriteString("\n")
	}
	for i := 0; i < shared; i++ {
		entry("a.go", "b.go")
	}
	for i := 0; i < alone; i++ {
		entry("a.go")
	}
	for i := 0; i < alone; i++ {
		entry("b.go")
	}
	return b.String()
}

// TestE2E_Coupling_WarnsWhenAllFiltered drives coupling end-to-end on a log whose
// only pair sits below the default threshold: stdout is the empty-but-valid
// envelope (exit 0) and stderr carries exactly one JSON warning line naming the
// coupling_all_filtered code and the highest observed degree.
func TestE2E_Coupling_WarnsWhenAllFiltered(t *testing.T) {
	log := weakCouplingLog(5, 12)

	var stdout, stderr bytes.Buffer
	code := run([]string{"codelens", "coupling"}, strings.NewReader(log), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", code, stderr.String())
	}

	var env struct {
		OK       bool `json:"ok"`
		RowCount int  `json:"row_count"`
		Rows     []struct {
			Degree int `json:"degree"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout.String())
	}
	if !env.OK || env.RowCount != 0 || len(env.Rows) != 0 {
		t.Fatalf("envelope = ok:%v row_count:%d rows:%d, want ok:true 0/0", env.OK, env.RowCount, len(env.Rows))
	}

	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stderr lines = %d, want 1:\n%s", len(lines), stderr.String())
	}
	var warn struct {
		Level   string `json:"level"`
		Code    string `json:"code"`
		Details struct {
			MaxDegree   int `json:"max_degree"`
			MinCoupling int `json:"min_coupling"`
		} `json:"details"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &warn); err != nil {
		t.Fatalf("stderr is not a JSON warning: %v\n%s", err, lines[0])
	}
	if warn.Level != "warning" {
		t.Errorf("level = %q, want warning", warn.Level)
	}
	if warn.Code != "coupling_all_filtered" {
		t.Errorf("code = %q, want coupling_all_filtered", warn.Code)
	}
	if warn.Details.MaxDegree != 29 {
		t.Errorf("details.max_degree = %d, want 29", warn.Details.MaxDegree)
	}
	if warn.Details.MinCoupling != 30 {
		t.Errorf("details.min_coupling = %d, want 30", warn.Details.MinCoupling)
	}
}
