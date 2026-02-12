package tasks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jaredpalmer/mogcli/internal/errfmt"
	"github.com/jaredpalmer/mogcli/internal/graph"
)

func TestListTasksUsesPageTokenURL(t *testing.T) {
	t.Parallel()

	requests := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/tasks/next" {
			t.Fatalf("expected /tasks/next path, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "state=abc" {
			t.Fatalf("expected resume query, got %q", r.URL.RawQuery)
		}
		_, _ = fmt.Fprintf(w, `{"value":[{"id":"1"},{"id":"2"}],"@odata.nextLink":"%s/tasks/next?state=next"}`, serverURL)
	}))
	defer server.Close()
	serverURL = server.URL

	client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
	client.BaseURL = serverURL
	client.HTTPClient = server.Client()

	svc := New(client)
	items, next, err := svc.ListTasks(context.Background(), "ignored-list-id", 1, serverURL+"/tasks/next?state=abc")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item due to max cap, got %d", len(items))
	}
	if next != serverURL+"/tasks/next?state=next" {
		t.Fatalf("unexpected next link: %s", next)
	}
}

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
