package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/errfmt"
	"github.com/jaredpalmer/mogcli/internal/graph"
)

var listTaskScopes = []string{"Tasks.Read"}
var getTaskScopes = []string{"Tasks.Read"}
var createTaskScopes = []string{"Tasks.ReadWrite"}
var updateTaskScopes = []string{"Tasks.ReadWrite"}
var deleteTaskScopes = []string{"Tasks.ReadWrite"}

type Service struct {
	client *graph.Client
}

func New(client *graph.Client) *Service {
	return &Service{client: client}
}

func (s *Service) Lists(ctx context.Context) ([]map[string]any, error) {
	_, body, err := s.client.Do(ctx, http.MethodGet, "/me/todo/lists", nil, nil, listTaskScopes, nil)
	if err != nil {
		return nil, err
	}
	items, _, err := decodeValuePage(body)
	return items, err
}

func (s *Service) ListTasks(ctx context.Context, listID string, max int, page string) ([]map[string]any, string, error) {
	query := url.Values{}
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks"
	if strings.TrimSpace(page) != "" {
		path = strings.TrimSpace(page)
		query = nil
	}

	_, body, err := s.client.Do(ctx, http.MethodGet, path, query, nil, listTaskScopes, nil)
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

func (s *Service) GetTask(ctx context.Context, listID string, taskID string) (map[string]any, error) {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	var payload map[string]any
	err := s.client.DoJSON(ctx, http.MethodGet, path, nil, nil, getTaskScopes, &payload)
	return payload, err
}

func (s *Service) CreateTask(ctx context.Context, listID string, payload map[string]any) (map[string]any, error) {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks"
	var created map[string]any
	err := s.client.DoJSON(ctx, http.MethodPost, path, nil, payload, createTaskScopes, &created)
	return created, normalizeTaskMutationError(err)
}

func (s *Service) UpdateTask(ctx context.Context, listID string, taskID string, payload map[string]any) error {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	_, _, err := s.client.Do(ctx, http.MethodPatch, path, nil, payload, updateTaskScopes, nil)
	return normalizeTaskMutationError(err)
}

func (s *Service) DeleteTask(ctx context.Context, listID string, taskID string) error {
	path := "/me/todo/lists/" + url.PathEscape(strings.TrimSpace(listID)) + "/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	_, _, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil, deleteTaskScopes, nil)
	return normalizeTaskMutationError(err)
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

func normalizeTaskMutationError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *graph.APIError
	if !errors.As(err, &apiErr) {
		return err
	}

	if apiErr.Status != http.StatusBadRequest && apiErr.Status != http.StatusForbidden {
		return err
	}

	if !isTaskMutabilityError(apiErr.Code, apiErr.Message) {
		return err
	}

	return errfmt.NewUserFacingError(
		"Built-in Microsoft To Do lists have mutability limits; use a custom list (`wellknownListName=none`) or another list.",
		err,
	)
}

func isTaskMutabilityError(code string, message string) bool {
	lowerCode := strings.ToLower(strings.TrimSpace(code))
	lowerMessage := strings.ToLower(strings.TrimSpace(message))

	if strings.Contains(lowerCode, "readonly") || strings.Contains(lowerCode, "immutable") {
		return true
	}

	if strings.Contains(lowerMessage, "wellknownlistname") {
		return true
	}

	if strings.Contains(lowerMessage, "well-known") && strings.Contains(lowerMessage, "list") {
		return true
	}

	if strings.Contains(lowerMessage, "built-in") && strings.Contains(lowerMessage, "list") {
		return true
	}

	if strings.Contains(lowerMessage, "read-only") && strings.Contains(lowerMessage, "list") {
		return true
	}

	if strings.Contains(lowerMessage, "immutable") && strings.Contains(lowerMessage, "list") {
		return true
	}

	if strings.Contains(lowerMessage, "list") &&
		(strings.Contains(lowerMessage, "cannot update") ||
			strings.Contains(lowerMessage, "cannot be updated") ||
			strings.Contains(lowerMessage, "cannot modify") ||
			strings.Contains(lowerMessage, "cannot delete")) {
		return true
	}

	return false
}

func trimPage(items []map[string]any, next string, max int) ([]map[string]any, string) {
	if max <= 0 || len(items) <= max {
		return items, next
	}
	return items[:max], next
}
