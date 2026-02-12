package contacts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jared/mogcli/internal/graph"
)

var DelegatedScopes = []string{"Contacts.Read", "Contacts.ReadWrite"}

type Service struct {
	client *graph.Client
}

func New(client *graph.Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, max int, page string) ([]map[string]any, string, error) {
	query := url.Values{}
	query.Set("$select", "id,displayName,emailAddresses,mobilePhone")
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	endpoint := "/me/contacts"
	if strings.TrimSpace(page) != "" {
		endpoint = strings.TrimSpace(page)
		query = nil
	}

	_, body, err := s.client.Do(ctx, http.MethodGet, endpoint, query, nil, DelegatedScopes, nil)
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
	err := s.client.DoJSON(ctx, http.MethodGet, "/me/contacts/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, DelegatedScopes, &payload)
	return payload, err
}

func (s *Service) Create(ctx context.Context, payload map[string]any) (map[string]any, error) {
	var created map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, "/me/contacts", nil, payload, DelegatedScopes, &created)
	return created, err
}

func (s *Service) Update(ctx context.Context, id string, payload map[string]any) error {
	_, _, err := s.client.Do(ctx, http.MethodPatch, "/me/contacts/"+url.PathEscape(strings.TrimSpace(id)), nil, payload, DelegatedScopes, nil)
	return err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	_, _, err := s.client.Do(ctx, http.MethodDelete, "/me/contacts/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, DelegatedScopes, nil)
	return err
}

func decodeValuePage(body []byte) ([]map[string]any, string, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, "", fmt.Errorf("decode json response: %w", err)
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

func trimPage(items []map[string]any, next string, max int) ([]map[string]any, string) {
	if max <= 0 || len(items) <= max {
		return items, next
	}
	return items[:max], next
}
