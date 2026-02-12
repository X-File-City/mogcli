package tasks

import (
	"errors"
	"testing"

	"github.com/jared/mogcli/internal/errfmt"
	"github.com/jared/mogcli/internal/graph"
)

func TestNormalizeTaskMutationErrorMapsMutabilityGraphError(t *testing.T) {
	t.Parallel()

	input := &graph.APIError{
		Status:  400,
		Code:    "ErrorInvalidRequest",
		Message: "The built-in list cannot be updated.",
	}

	mapped := normalizeTaskMutationError(input)
	if mapped == nil {
		t.Fatal("expected mapped error, got nil")
	}
	if mapped == input {
		t.Fatal("expected mapped user-facing error, got original error")
	}

	var userErr *errfmt.UserFacingError
	if !errors.As(mapped, &userErr) {
		t.Fatalf("expected UserFacingError, got %T", mapped)
	}
	if userErr.Cause != input {
		t.Fatalf("expected cause to be original API error, got %#v", userErr.Cause)
	}
}

func TestNormalizeTaskMutationErrorPassesUnrelatedGraphError(t *testing.T) {
	t.Parallel()

	input := &graph.APIError{
		Status:  403,
		Code:    "AccessDenied",
		Message: "Access denied",
	}

	mapped := normalizeTaskMutationError(input)
	if mapped != input {
		t.Fatalf("expected original error, got %#v", mapped)
	}
}

func TestNormalizeTaskMutationErrorPassesNonGraphError(t *testing.T) {
	t.Parallel()

	input := errors.New("network timeout")
	mapped := normalizeTaskMutationError(input)
	if mapped != input {
		t.Fatalf("expected original error, got %#v", mapped)
	}
}
