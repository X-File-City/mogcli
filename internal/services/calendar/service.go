package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jared/mogcli/internal/graph"
)

var DelegatedScopes = []string{"Calendars.Read", "Calendars.ReadWrite"}

type Service struct {
	client *graph.Client
}

func New(client *graph.Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, from string, to string, max int) ([]map[string]any, string, error) {
	query := url.Values{}
	query.Set("startDateTime", from)
	query.Set("endDateTime", to)
	query.Set("$select", "id,subject,start,end,organizer")
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	headers := http.Header{}
	headers.Set("Prefer", `outlook.timezone="UTC"`)

	_, body, err := s.client.Do(ctx, http.MethodGet, "/me/calendarView", query, nil, DelegatedScopes, headers)
	if err != nil {
		return nil, "", err
	}

	return decodeValuePage(body)
}

func (s *Service) Get(ctx context.Context, id string) (map[string]any, error) {
	var payload map[string]any
	err := s.client.DoJSON(ctx, http.MethodGet, "/me/events/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, DelegatedScopes, &payload)
	return payload, err
}

func (s *Service) Create(ctx context.Context, payload map[string]any) (map[string]any, error) {
	var created map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, "/me/events", nil, payload, DelegatedScopes, &created)
	return created, err
}

func (s *Service) Update(ctx context.Context, id string, payload map[string]any) error {
	_, _, err := s.client.Do(ctx, http.MethodPatch, "/me/events/"+url.PathEscape(strings.TrimSpace(id)), nil, payload, DelegatedScopes, nil)
	return err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	_, _, err := s.client.Do(ctx, http.MethodDelete, "/me/events/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, DelegatedScopes, nil)
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
