package mail

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jared/mogcli/internal/graph"
)

func TestListUsesPageTokenURLAndSearchHeader(t *testing.T) {
	t.Parallel()

	requests := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/mail/next" {
			t.Fatalf("expected /mail/next path, got %s", r.URL.Path)
		}
		if r.Header.Get("ConsistencyLevel") != "eventual" {
			t.Fatalf("expected ConsistencyLevel eventual header, got %q", r.Header.Get("ConsistencyLevel"))
		}
		_, _ = fmt.Fprintf(w, `{"value":[{"id":"1"},{"id":"2"}],"@odata.nextLink":"%s/mail/next?state=next"}`, serverURL)
	}))
	defer server.Close()
	serverURL = server.URL

	client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
	client.BaseURL = serverURL
	client.HTTPClient = server.Client()

	svc := New(client)
	page := serverURL + "/mail/next?%24search=%22foo%22&state=abc"
	items, next, err := svc.List(context.Background(), 1, "", page)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item due to max cap, got %d", len(items))
	}
	if next != serverURL+"/mail/next?state=next" {
		t.Fatalf("unexpected next link: %s", next)
	}
}
