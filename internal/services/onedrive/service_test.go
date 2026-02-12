package onedrive

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
		if r.URL.Path != "/resume" {
			t.Fatalf("expected /resume path, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "state=abc" {
			t.Fatalf("expected resume query, got %q", r.URL.RawQuery)
		}
		_, _ = fmt.Fprintf(w, `{"value":[{"id":"1"},{"id":"2"}],"@odata.nextLink":"%s/resume?state=next"}`, serverURL)
	}))
	defer server.Close()
	serverURL = server.URL

	client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
	client.BaseURL = serverURL
	client.HTTPClient = server.Client()

	svc := New(client, "")
	items, next, err := svc.List(context.Background(), "/ignored", 1, serverURL+"/resume?state=abc")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item due to max cap, got %d", len(items))
	}
	if next != serverURL+"/resume?state=next" {
		t.Fatalf("unexpected next link: %s", next)
	}
}

func TestEndpointsRouteByAuthMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		user       string
		basePrefix string
	}{
		{
			name:       "delegated uses me endpoints",
			user:       "",
			basePrefix: "/me/drive",
		},
		{
			name:       "app-only uses user endpoints",
			user:       "person@example.com",
			basePrefix: "/users/" + url.PathEscape("person@example.com") + "/drive",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == tc.basePrefix+"/root/children":
					_, _ = fmt.Fprint(w, `{"value":[]}`)
				case r.Method == http.MethodGet && r.URL.Path == tc.basePrefix+"/root:/docs/report.txt:/content":
					_, _ = fmt.Fprint(w, `hello`)
				case r.Method == http.MethodPut && r.URL.Path == tc.basePrefix+"/root:/docs/report.txt:/content":
					w.WriteHeader(http.StatusCreated)
				case r.Method == http.MethodPost && r.URL.Path == tc.basePrefix+"/root:/projects:/children":
					w.WriteHeader(http.StatusCreated)
				case r.Method == http.MethodDelete && r.URL.Path == tc.basePrefix+"/root:/projects/old":
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
			if _, _, err := svc.List(context.Background(), "/", 10, ""); err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if _, err := svc.Get(context.Background(), "/docs/report.txt"); err != nil {
				t.Fatalf("Get failed: %v", err)
			}
			if err := svc.Put(context.Background(), "/docs/report.txt", []byte("hello")); err != nil {
				t.Fatalf("Put failed: %v", err)
			}
			if err := svc.Mkdir(context.Background(), "/projects/new"); err != nil {
				t.Fatalf("Mkdir failed: %v", err)
			}
			if err := svc.Remove(context.Background(), "/projects/old"); err != nil {
				t.Fatalf("Remove failed: %v", err)
			}
		})
	}
}
