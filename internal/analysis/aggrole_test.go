package analysis

import "testing"

func TestValidAggRole(t *testing.T) {
	for _, r := range AggRoles() {
		if !ValidAggRole(r) {
			t.Errorf("ValidAggRole(%q) = false, want true for a declared role", r)
		}
	}
	// signed-additive is Flint's fifth role, deliberately omitted: codelens has
	// no signed measure (added and deleted are separate non-negative columns). A
	// future author who adds a net-churn column must remove it from this negative
	// list in the same change that declares it (see ADR 0008's reachable-only
	// rule), rather than being confused by a test that appears to forbid it.
	for _, r := range []AggRole{"", "signed-additive", "ADDITIVE", "measure", "unknown"} {
		if ValidAggRole(r) {
			t.Errorf("ValidAggRole(%q) = true, want false", r)
		}
	}
}

func TestAggRoles_Closed(t *testing.T) {
	want := []AggRole{AggAdditive, AggIntensive, AggDimension, AggIdentifier}
	got := AggRoles()
	if len(got) != len(want) {
		t.Fatalf("AggRoles() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AggRoles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAggRoleOf_Exhaustive is the vocabulary's safety property: every declared
// semantic maps to a declared role. AggRoleOf is a switch with no default, so a
// new semantic that is not given a role returns the zero value and fails here.
func TestAggRoleOf_Exhaustive(t *testing.T) {
	for _, s := range Semantics() {
		if r := AggRoleOf(s); !ValidAggRole(r) {
			t.Errorf("semantic %q has no declared aggregation role", s)
		}
	}
}

func TestAggRoleOf_Assignments(t *testing.T) {
	want := map[Semantic]AggRole{
		SemanticCount:          AggAdditive,
		SemanticLoc:            AggAdditive,
		SemanticPercentage:     AggIntensive,
		SemanticRatio:          AggIntensive,
		SemanticDurationMonths: AggIntensive,
		SemanticFilepath:       AggDimension,
		SemanticPerson:         AggDimension,
		SemanticDate:           AggDimension,
		SemanticLabel:          AggDimension,
		SemanticFlag:           AggDimension,
		SemanticCommitID:       AggIdentifier,
		SemanticText:           AggIdentifier,
	}
	for s, r := range want {
		if got := AggRoleOf(s); got != r {
			t.Errorf("AggRoleOf(%q) = %q, want %q", s, got, r)
		}
	}
}

func TestAggRoleOf_Unknown(t *testing.T) {
	if got := AggRoleOf("nonsense"); got != "" {
		t.Errorf("AggRoleOf(%q) = %q, want \"\"", "nonsense", got)
	}
}
