package version

import "testing"

func TestCurrentPrefersOverride(t *testing.T) {
	orig := Override
	t.Cleanup(func() { Override = orig })

	Override = "v1.2.3"
	if got := Current(); got != "v1.2.3" {
		t.Fatalf("Current() = %q, want %q", got, "v1.2.3")
	}
}

func TestCurrentFallsBackWhenNoOverride(t *testing.T) {
	orig := Override
	t.Cleanup(func() { Override = orig })

	Override = ""
	if got := Current(); got == "" {
		t.Fatal("Current() returned an empty string, want a non-empty version")
	}
}
