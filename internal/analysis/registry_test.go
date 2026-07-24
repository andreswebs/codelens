package analysis

import (
	"testing"

	"github.com/andreswebs/codelens/internal/model"
)

// resetRegistry clears the package-global registry so each test starts from a
// known-empty state; the global is an implementation detail these in-package
// tests are allowed to reach.
func resetRegistry() {
	mu.Lock()
	defer mu.Unlock()
	descriptors = map[string]Descriptor{}
	byKey = map[string]string{}
}

// stub returns a minimal descriptor whose Run echoes the analysis name, enough
// to assert identity through the registry.
func stub(name string, aliases ...string) Descriptor {
	return Descriptor{
		Name:    name,
		Aliases: aliases,
		Summary: name + " summary",
		Run: func(_ []model.Modification, _ Opts) (any, error) {
			return nil, nil
		},
	}
}

func TestRegister_LookupByName(t *testing.T) {
	resetRegistry()
	Register(stub("authors"))

	got, ok := Lookup("authors")
	if !ok {
		t.Fatal("Lookup(\"authors\") not found after Register")
	}
	if got.Name != "authors" {
		t.Errorf("Name = %q, want %q", got.Name, "authors")
	}
	if got.Summary != "authors summary" {
		t.Errorf("Summary = %q, want %q", got.Summary, "authors summary")
	}
}

func TestRegister_LookupByAlias(t *testing.T) {
	resetRegistry()
	Register(stub("sum-of-coupling", "soc"))

	got, ok := Lookup("soc")
	if !ok {
		t.Fatal("Lookup(\"soc\") not found; alias did not resolve")
	}
	if got.Name != "sum-of-coupling" {
		t.Errorf("alias resolved to Name = %q, want %q", got.Name, "sum-of-coupling")
	}

	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup(\"nope\") = ok, want not found")
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	t.Run("duplicate name", func(t *testing.T) {
		resetRegistry()
		Register(stub("authors"))
		defer func() {
			if recover() == nil {
				t.Error("registering a duplicate name did not panic")
			}
		}()
		Register(stub("authors"))
	})

	t.Run("alias collides with existing name", func(t *testing.T) {
		resetRegistry()
		Register(stub("authors"))
		defer func() {
			if recover() == nil {
				t.Error("registering an alias colliding with a name did not panic")
			}
		}()
		Register(stub("revisions", "authors"))
	})

	t.Run("alias collides with existing alias", func(t *testing.T) {
		resetRegistry()
		Register(stub("sum-of-coupling", "soc"))
		defer func() {
			if recover() == nil {
				t.Error("registering a duplicate alias did not panic")
			}
		}()
		Register(stub("coupling", "soc"))
	})
}

func TestAll_SortedCopy(t *testing.T) {
	resetRegistry()
	Register(stub("revisions"))
	Register(stub("authors"))
	Register(stub("coupling"))

	all := All()
	want := []string{"authors", "coupling", "revisions"}
	if len(all) != len(want) {
		t.Fatalf("All() len = %d, want %d", len(all), len(want))
	}
	for i, name := range want {
		if all[i].Name != name {
			t.Errorf("All()[%d].Name = %q, want %q", i, all[i].Name, name)
		}
	}

	all[0] = Descriptor{Name: "mutated"}
	again := All()
	if again[0].Name != "authors" {
		t.Errorf("mutating returned slice affected registry: got %q, want %q", again[0].Name, "authors")
	}
}
