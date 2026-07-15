package gitlog

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// update, when set via `go test -update`, regenerates the golden files from the
// current parser output instead of asserting against them. Regenerated goldens
// must be reviewed by hand before being committed.
var update = flag.Bool("update", false, "regenerate gitlog golden files")

// goldenFixtures enumerates the ported git2 log fixtures under testdata. Each
// name maps to testdata/<name>.log (input) and testdata/<name>.golden.json (the
// expected []model.Modification). Origin and licensing are noted in
// testdata/README.md.
var goldenFixtures = []string{
	"entry",
	"binary",
	"entries",
	"pull_requests",
	"simple_git2",
}

// TestParse_Golden parses each testdata fixture and compares the resulting
// modification records against its committed golden JSON. Run with -update to
// regenerate the goldens after an intentional change.
func TestParse_Golden(t *testing.T) {
	for _, name := range goldenFixtures {
		t.Run(name, func(t *testing.T) {
			logPath := filepath.Join("testdata", name+".log")
			goldenPath := filepath.Join("testdata", name+".golden.json")

			in, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			mods, err := ParseString(string(in), model.Options{})
			if err != nil {
				t.Fatalf("ParseString(%s): %v", logPath, err)
			}

			got, err := json.MarshalIndent(mods, "", "  ")
			if err != nil {
				t.Fatalf("marshal golden: %v", err)
			}
			got = append(got, '\n')

			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run `go test -update` to create it): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("parsed records do not match %s\n got:\n%s\nwant:\n%s",
					goldenPath, got, want)
			}
		})
	}
}

// TestGoldens_Reviewed guards against accidental golden regeneration drift: the
// entries fixture is fixed at exactly six modification records (two commits of
// two and four files). If a future edit or an unreviewed -update changes the
// record count, this fails independently of the byte-level golden comparison.
func TestGoldens_Reviewed(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "entries.log"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	mods, err := ParseString(string(in), model.Options{})
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}

	if len(mods) != 6 {
		t.Fatalf("entries.log parsed to %d records, want 6", len(mods))
	}
}
