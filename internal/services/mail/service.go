package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/graph"
)

var listMailScopes = []string{"Mail.Read"}
var getMailScopes = []string{"Mail.Read"}
var sendMailScopes = []string{"Mail.Send"}

type Service struct {
	client      *graph.Client
	appOnlyUser string
}

func New(client *graph.Client, appOnlyUser string) *Service {
	return &Service{client: client, appOnlyUser: strings.TrimSpace(appOnlyUser)}
}

func (s *Service) List(ctx context.Context, max int, queryText string, page string) ([]map[string]any, string, error) {
	query := url.Values{}
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}
	query.Set("$select", "id,subject,receivedDateTime,isRead,from")

	headers := http.Header{}
	if strings.TrimSpace(queryText) != "" {
		query.Set("$search", fmt.Sprintf("\"%s\"", strings.TrimSpace(queryText)))
		headers.Set("ConsistencyLevel", "eventual")
	}

	endpoint := s.messagesEndpoint()
	if strings.TrimSpace(page) != "" {
		endpoint = strings.TrimSpace(page)
		query = nil
		if hasSearchQuery(page) {
			headers.Set("ConsistencyLevel", "eventual")
		}
	}

	_, body, err := s.client.Do(ctx, http.MethodGet, endpoint, query, nil, listMailScopes, headers)
	if err != nil {
		return nil, "", err
	}

	items, next, err := decodeValuePage(body)
	if err != nil {
		return nil, "", err
	}

	trimmed, trimmedNext := trimPage(items, next, max)
	return trimmed, trimmedNext, nil
}

func (s *Service) Get(ctx context.Context, id string) (map[string]any, error) {
	var payload map[string]any
	err := s.client.DoJSON(ctx, http.MethodGet, s.messagesEndpoint()+"/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, getMailScopes, &payload)
	return payload, err
}

func (s *Service) Send(ctx context.Context, to []string, subject string, body string) error {
	toRecipients := make([]map[string]any, 0, len(to))
	for _, address := range to {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		toRecipients = append(toRecipients, map[string]any{
			"emailAddress": map[string]any{"address": address},
		})
	}

	payload := map[string]any{
		"message": map[string]any{
			"subject": subject,
			"body": map[string]any{
				"contentType": "Text",
				"content":     body,
			},
			"toRecipients": toRecipients,
		},
		"saveToSentItems": true,
	}

	_, _, err := s.client.Do(ctx, http.MethodPost, s.sendMailEndpoint(), nil, payload, sendMailScopes, nil)
	return err
}

func (s *Service) messagesEndpoint() string {
	if s.appOnlyUser != "" {
		return "/users/" + url.PathEscape(s.appOnlyUser) + "/messages"
	}
	return "/me/messages"
}

func (s *Service) sendMailEndpoint() string {
	if s.appOnlyUser != "" {
		return "/users/" + url.PathEscape(s.appOnlyUser) + "/sendMail"
	}
	return "/me/sendMail"
}

func decodeValuePage(body []byte) ([]map[string]any, string, error) {
	var payload map[string]any
	if err := jsonUnmarshal(body, &payload); err != nil {
		return nil, "", err
	}

	items := make([]map[string]any, 0)
	if values, ok := payload["value"].([]any); ok {
		for _, value := range values {
			if item, ok := value.(map[string]any); ok {
				items = append(items, item)
			}
		}
	}

	next, _ := payload["@odata.nextLink"].(string)
	return items, strings.TrimSpace(next), nil
}

func jsonUnmarshal(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode json response: %w", err)
	}
	return nil
}

func trimPage(items []map[string]any, next string, max int) ([]map[string]any, string) {
	if max <= 0 || len(items) <= max {
		return items, next
	}
	return items[:max], next
}

func hasSearchQuery(page string) bool {
	u, err := url.Parse(strings.TrimSpace(page))
	if err != nil {
		return false
	}
	return strings.TrimSpace(u.Query().Get("$search")) != ""
}
