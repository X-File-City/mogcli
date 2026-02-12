package contacts

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jaredpalmer/mogcli/internal/graph"
)

func TestListUsesPageTokenURL(t *testing.T) {
	t.Parallel()

	requests := 0
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/contacts/next" {
			t.Fatalf("expected /contacts/next path, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "state=abc" {
			t.Fatalf("expected resume query, got %q", r.URL.RawQuery)
		}
		_, _ = fmt.Fprintf(w, `{"value":[{"id":"1"},{"id":"2"}],"@odata.nextLink":"%s/contacts/next?state=next"}`, serverURL)
	}))
	defer server.Close()
	serverURL = server.URL

	client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
	client.BaseURL = serverURL
	client.HTTPClient = server.Client()

	svc := New(client, "")
	items, next, err := svc.List(context.Background(), 1, serverURL+"/contacts/next?state=abc")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item due to max cap, got %d", len(items))
	}
	if next != serverURL+"/contacts/next?state=next" {
		t.Fatalf("unexpected next link: %s", next)
	}
}

func TestEndpointsRouteByAuthMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		user         string
		wantBasePath string
		wantItemPath string
	}{
		{
			name:         "delegated uses me endpoints",
			user:         "",
			wantBasePath: "/me/contacts",
			wantItemPath: "/me/contacts/contact-id",
		},
		{
			name:         "app-only uses user endpoints",
			user:         "person@example.com",
			wantBasePath: "/users/" + url.PathEscape("person@example.com") + "/contacts",
			wantItemPath: "/users/" + url.PathEscape("person@example.com") + "/contacts/contact-id",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == tc.wantBasePath:
					_, _ = fmt.Fprint(w, `{"value":[]}`)
				case r.Method == http.MethodGet && r.URL.Path == tc.wantItemPath:
					_, _ = fmt.Fprint(w, `{"id":"contact-id"}`)
				case r.Method == http.MethodPost && r.URL.Path == tc.wantBasePath:
					_, _ = fmt.Fprint(w, `{"id":"contact-id"}`)
				case r.Method == http.MethodPatch && r.URL.Path == tc.wantItemPath:
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodDelete && r.URL.Path == tc.wantItemPath:
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
			client.BaseURL = server.URL
			client.HTTPClient = server.Client()

			svc := New(client, tc.user)
			if _, _, err := svc.List(context.Background(), 10, ""); err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if _, err := svc.Get(context.Background(), "contact-id"); err != nil {
				t.Fatalf("Get failed: %v", err)
			}
			if _, err := svc.Create(context.Background(), map[string]any{"displayName": "Name"}); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
			if err := svc.Update(context.Background(), "contact-id", map[string]any{"displayName": "Updated"}); err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			if err := svc.Delete(context.Background(), "contact-id"); err != nil {
				t.Fatalf("Delete failed: %v", err)
			}
		})
	}
}
