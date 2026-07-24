package terr_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/andreswebs/codelens/internal/terr"
)

func TestNew_Accessors(t *testing.T) {
	e := terr.New("c", 3, "h", "m")
	if got := e.Code(); got != "c" {
		t.Errorf("Code() = %q, want %q", got, "c")
	}
	if got := e.ExitCode(); got != 3 {
		t.Errorf("ExitCode() = %d, want %d", got, 3)
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
	e := terr.New("c", 3, "h", "m").Wrap(inner)
	if got := e.Error(); got != "m: boom" {
		t.Errorf("Error() = %q, want %q", got, "m: boom")
	}
}

func TestErrorsAs_RecoversCoded(t *testing.T) {
	base := terr.New("parse_error", 3, "h", "failed")
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
	e := terr.New("c", 3, "h", "m").Wrap(inner)
	if got := errors.Unwrap(e); got != inner {
		t.Errorf("errors.Unwrap() = %v, want %v", got, inner)
	}

	base := terr.New("parse_error", 3, "h", "failed")
	chain := fmt.Errorf("%w: ctx", base)
	if !errors.Is(chain, base) {
		t.Errorf("errors.Is(chain, base) = false, want true")
	}
}

func TestWithDetails_ImplementsDetailed(t *testing.T) {
	base := terr.New("parse_error", 3, "h", "failed")
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
