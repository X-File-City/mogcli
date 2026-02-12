package groups

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jared/mogcli/internal/graph"
)

var DelegatedScopes = []string{"Group.Read.All", "GroupMember.Read.All"}

type Service struct {
	client *graph.Client
}

func New(client *graph.Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, max int) ([]map[string]any, string, error) {
	query := url.Values{}
	query.Set("$select", "id,displayName,mail,mailEnabled,securityEnabled")
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	_, body, err := s.client.Do(ctx, http.MethodGet, "/groups", query, nil, DelegatedScopes, nil)
	if err != nil {
		return nil, "", err
	}

	return decodeValuePage(body)
}

func (s *Service) Get(ctx context.Context, id string) (map[string]any, error) {
	var payload map[string]any
	err := s.client.DoJSON(ctx, http.MethodGet, "/groups/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, DelegatedScopes, &payload)
	return payload, err
}

func (s *Service) Members(ctx context.Context, id string, max int) ([]map[string]any, string, error) {
	query := url.Values{}
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	_, body, err := s.client.Do(ctx, http.MethodGet, "/groups/"+url.PathEscape(strings.TrimSpace(id))+"/members", query, nil, DelegatedScopes, nil)
	if err != nil {
		return nil, "", err
	}

	return decodeValuePage(body)
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
