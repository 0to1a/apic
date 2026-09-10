package apic

import (
	"fmt"
	"testing"
)

func TestStatus_ErrorString(t *testing.T) {
	st := Error(NotFound, "post not found")
	if got, want := st.Error(), "NOT_FOUND: post not found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestStatus_Errorf(t *testing.T) {
	st := Errorf(InvalidArgument, "bad field %q", "id")
	if got, want := st.Message, `bad field "id"`; got != want {
		t.Errorf("Message = %q, want %q", got, want)
	}
}

func TestStatus_WithDetailsChains(t *testing.T) {
	st := Error(InvalidArgument, "missing fields").WithDetails(map[string]any{"id": "required"})
	if st.Details["id"] != "required" {
		t.Errorf("Details = %v", st.Details)
	}
}

func TestResolveError(t *testing.T) {
	original := Error(NotFound, "post not found")
	if got := ResolveError(original); got != original {
		t.Errorf("ResolveError of a *Status should return it unchanged, got %v", got)
	}
	generic := ResolveError(fmt.Errorf("boom"))
	if generic.Code != Internal {
		t.Errorf("ResolveError of a plain error should be Internal, got %v", generic.Code)
	}
}
