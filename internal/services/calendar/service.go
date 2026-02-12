package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/graph"
)

var listCalendarScopes = []string{"Calendars.Read"}
var getCalendarScopes = []string{"Calendars.Read"}
var createCalendarScopes = []string{"Calendars.ReadWrite"}
var updateCalendarScopes = []string{"Calendars.ReadWrite"}
var deleteCalendarScopes = []string{"Calendars.ReadWrite"}

type Service struct {
	client *graph.Client
}

func New(client *graph.Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, from string, to string, max int, page string) ([]map[string]any, string, error) {
	query := url.Values{}
	query.Set("startDateTime", from)
	query.Set("endDateTime", to)
	query.Set("$select", "id,subject,start,end,organizer")
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	headers := http.Header{}
	headers.Set("Prefer", `outlook.timezone="UTC"`)

	endpoint := "/me/calendarView"
	if strings.TrimSpace(page) != "" {
		endpoint = strings.TrimSpace(page)
		query = nil
	}

	_, body, err := s.client.Do(ctx, http.MethodGet, endpoint, query, nil, listCalendarScopes, headers)
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
	err := s.client.DoJSON(ctx, http.MethodGet, "/me/events/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, getCalendarScopes, &payload)
	return payload, err
}

func (s *Service) Create(ctx context.Context, payload map[string]any) (map[string]any, error) {
	var created map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, "/me/events", nil, payload, createCalendarScopes, &created)
	return created, err
}

func (s *Service) Update(ctx context.Context, id string, payload map[string]any) error {
	_, _, err := s.client.Do(ctx, http.MethodPatch, "/me/events/"+url.PathEscape(strings.TrimSpace(id)), nil, payload, updateCalendarScopes, nil)
	return err
}

func (s *Service) Delete(ctx context.Context, id string) error {
	_, _, err := s.client.Do(ctx, http.MethodDelete, "/me/events/"+url.PathEscape(strings.TrimSpace(id)), nil, nil, deleteCalendarScopes, nil)
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
