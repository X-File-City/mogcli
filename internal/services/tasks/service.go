package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jared/mogcli/internal/graph"
)

var DelegatedScopes = []string{"Tasks.Read", "Tasks.ReadWrite"}

type Service struct {
	client *graph.Client
}

func New(client *graph.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Lists(ctx context.Context) ([]map[string]any, error) {
	_, body, err := s.client.Do(ctx, http.MethodGet, "/me/todo/lists", nil, nil, DelegatedScopes, nil)
	if err != nil {
		return nil, err
	}
	items, _, err := decodeValuePage(body)
	return items, err
}

func (s *Service) ListTasks(ctx context.Context, listID string, max int) ([]map[string]any, string, error) {
	query := url.Values{}
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks"
	_, body, err := s.client.Do(ctx, http.MethodGet, path, query, nil, DelegatedScopes, nil)
	if err != nil {
		return nil, "", err
	}

	return decodeValuePage(body)
}

func (s *Service) GetTask(ctx context.Context, listID string, taskID string) (map[string]any, error) {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	var payload map[string]any
	err := s.client.DoJSON(ctx, http.MethodGet, path, nil, nil, DelegatedScopes, &payload)
	return payload, err
}

func (s *Service) CreateTask(ctx context.Context, listID string, payload map[string]any) (map[string]any, error) {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks"
	var created map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, path, nil, payload, DelegatedScopes, &created)
	return created, err
}

func (s *Service) UpdateTask(ctx context.Context, listID string, taskID string, payload map[string]any) error {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	_, _, err := s.client.Do(ctx, http.MethodPatch, path, nil, payload, DelegatedScopes, nil)
	return err
}

func (s *Service) DeleteTask(ctx context.Context, listID string, taskID string) error {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	_, _, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil, DelegatedScopes, nil)
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
