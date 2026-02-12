package mail

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jaredpalmer/mogcli/internal/graph"
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

	svc := New(client, "")
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

func TestEndpointsRouteByAuthMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		user             string
		wantListPath     string
		wantGetPath      string
		wantSendMailPath string
	}{
		{
			name:             "delegated uses me endpoints",
			user:             "",
			wantListPath:     "/me/messages",
			wantGetPath:      "/me/messages/message-id",
			wantSendMailPath: "/me/sendMail",
		},
		{
			name:             "app-only uses user endpoints",
			user:             "person@example.com",
			wantListPath:     "/users/" + url.PathEscape("person@example.com") + "/messages",
			wantGetPath:      "/users/" + url.PathEscape("person@example.com") + "/messages/message-id",
			wantSendMailPath: "/users/" + url.PathEscape("person@example.com") + "/sendMail",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == tc.wantListPath:
					_, _ = fmt.Fprint(w, `{"value":[]}`)
				case r.Method == http.MethodGet && r.URL.Path == tc.wantGetPath:
					_, _ = fmt.Fprint(w, `{"id":"message-id"}`)
				case r.Method == http.MethodPost && r.URL.Path == tc.wantSendMailPath:
					w.WriteHeader(http.StatusAccepted)
				default:
					t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			client := graph.NewClient(func(context.Context, []string) (string, error) { return "token", nil })
			client.BaseURL = server.URL
			client.HTTPClient = server.Client()

			svc := New(client, tc.user)
			if _, _, err := svc.List(context.Background(), 10, "", ""); err != nil {
				t.Fatalf("List failed: %v", err)
			}
			if _, err := svc.Get(context.Background(), "message-id"); err != nil {
				t.Fatalf("Get failed: %v", err)
			}
			if err := svc.Send(context.Background(), []string{"to@example.com"}, "subject", "body"); err != nil {
				t.Fatalf("Send failed: %v", err)
			}
		})
	}
}
