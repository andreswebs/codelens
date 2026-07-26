package terr_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/terr"
)

func TestNew_Accessors(t *testing.T) {
	e := terr.Newf("c", 64, "h", "m")
	if got := e.Code(); got != "c" {
		t.Errorf("Code() = %q, want %q", got, "c")
	}
	if got := e.ExitCode(); got != 64 {
		t.Errorf("ExitCode() = %d, want %d", got, 64)
	}
	if got := e.Hint(); got != "h" {
		t.Errorf("Hint() = %q, want %q", got, "h")
	}
	if got := e.Error(); got != "m" {
		t.Errorf("Error() = %q, want %q", got, "m")
	}
}

func TestError_WrappedMessage(t *testing.T) {
	inner := errors.New("boom")
	e := terr.Newf("c", 64, "h", "m").Wrap(inner)
	if got := e.Error(); got != "m: boom" {
		t.Errorf("Error() = %q, want %q", got, "m: boom")
	}
}

func TestErrorsAs_RecoversCoded(t *testing.T) {
	base := terr.Newf("parse_error", 65, "h", "failed")
	e := fmt.Errorf("%w: ctx", base)

	var c terr.Coded
	if !errors.As(e, &c) {
		t.Fatalf("errors.As did not recover a Coded from %v", e)
	}
	if c.Code() != base.Code() {
		t.Errorf("Code() = %q, want %q", c.Code(), base.Code())
	}
	if c.ExitCode() != base.ExitCode() {
		t.Errorf("ExitCode() = %d, want %d", c.ExitCode(), base.ExitCode())
	}
}

func TestUnwrap(t *testing.T) {
	inner := errors.New("inner")
	e := terr.Newf("c", 64, "h", "m").Wrap(inner)
	if got := errors.Unwrap(e); got != inner {
		t.Errorf("errors.Unwrap() = %v, want %v", got, inner)
	}

	base := terr.Newf("parse_error", 65, "h", "failed")
	chain := fmt.Errorf("%w: ctx", base)
	if !errors.Is(chain, base) {
		t.Errorf("errors.Is(chain, base) = false, want true")
	}
}

func TestWithDetails_ImplementsDetailed(t *testing.T) {
	base := terr.Newf("parse_error", 65, "h", "failed")
	details := map[string]any{"entry": 4}
	withDetails := base.WithDetails(details)

	var d terr.Detailed
	if !errors.As(error(withDetails), &d) {
		t.Fatalf("errors.As did not recover a Detailed")
	}
	if !reflect.DeepEqual(d.ErrorDetails(), details) {
		t.Errorf("ErrorDetails() = %v, want %v", d.ErrorDetails(), details)
	}

	if base.ErrorDetails() != nil {
		t.Errorf("base.ErrorDetails() = %v, want nil (copy semantics)", base.ErrorDetails())
	}
}

func TestAll_RegistrationOrderAndCopy(t *testing.T) {
	before := len(terr.All())
	a := terr.New("terr_test_order_alpha", 64, "", "alpha")
	b := terr.New("terr_test_order_beta", 65, "", "beta")

	got := terr.All()
	if len(got) != before+2 {
		t.Fatalf("All() length = %d, want %d", len(got), before+2)
	}
	if got[before] != a || got[before+1] != b {
		t.Errorf("All() tail = %v, want registration order [%v %v]", got[before:], a, b)
	}

	// The returned slice is a copy: mutating it must not affect a later All().
	got[before] = nil
	if again := terr.All(); again[before] != a {
		t.Errorf("All() returned a shared slice; mutation leaked (got %v, want %v)", again[before], a)
	}
}

func TestNewf_DoesNotRegister(t *testing.T) {
	before := len(terr.All())
	e := terr.Newf("terr_test_unregistered", 64, "", "value %q at %d", "x", 7)
	if after := len(terr.All()); after != before {
		t.Errorf("All() length changed after Newf: %d -> %d, want unchanged", before, after)
	}
	if got, want := e.Error(), `value "x" at 7`; got != want {
		t.Errorf("Newf message = %q, want %q", got, want)
	}
}

func TestIs_ThroughWrap(t *testing.T) {
	sentinel := terr.New("terr_test_is_wrap", 65, "", "boom")
	if !errors.Is(sentinel.Wrap(errors.New("inner")), sentinel) {
		t.Errorf("errors.Is(sentinel.Wrap(inner), sentinel) = false, want true")
	}
}

func TestIs_ThroughWithDetails(t *testing.T) {
	sentinel := terr.New("terr_test_is_details", 65, "", "boom")
	if !errors.Is(sentinel.WithDetails(map[string]any{"k": "v"}), sentinel) {
		t.Errorf("errors.Is(sentinel.WithDetails(x), sentinel) = false, want true")
	}
}

func TestIs_ThroughChainedCopies(t *testing.T) {
	sentinel := terr.New("terr_test_is_chain", 65, "", "boom")
	chained := sentinel.Wrap(errors.New("inner")).WithDetails(map[string]any{"k": "v"})
	if !errors.Is(chained, sentinel) {
		t.Errorf("errors.Is(sentinel.Wrap(inner).WithDetails(x), sentinel) = false, want true")
	}
}

func TestNew_PanicsOnDuplicateCode(t *testing.T) {
	const code = "terr_test_dup"
	_ = terr.New(code, 64, "", "first")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("New did not panic on a duplicate code")
		}
		if got, want := fmt.Sprint(r), `terr: duplicate error code "`+code+`"`; got != want {
			t.Errorf("panic = %q, want %q", got, want)
		}
	}()
	_ = terr.New(code, 64, "", "second")
}

func TestWrap_CopySemantics(t *testing.T) {
	base := terr.Newf("c", 64, "h", "m")
	if wrapped := base.Wrap(errors.New("inner")); errors.Unwrap(wrapped) == nil {
		t.Fatalf("Wrap did not attach the cause")
	}
	if errors.Unwrap(base) != nil {
		t.Errorf("Wrap mutated the receiver's cause; base is no longer reusable")
	}
}
