package onedrive

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/jared/mogcli/internal/graph"
)

var DelegatedScopes = []string{"Files.Read", "Files.ReadWrite"}

type Service struct {
	client *graph.Client
}

func New(client *graph.Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, remotePath string, max int) ([]map[string]any, string, error) {
	endpoint := "/me/drive/root/children"
	if cleaned := normalizeRemotePath(remotePath); cleaned != "/" {
		endpoint = "/me/drive/root:" + cleaned + ":/children"
	}

	query := url.Values{}
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	return s.client.Paginate(ctx, endpoint, query, DelegatedScopes, max)
}

func (s *Service) Get(ctx context.Context, remotePath string) ([]byte, error) {
	endpoint := "/me/drive/root:" + normalizeRemotePath(remotePath) + ":/content"
	_, body, err := s.client.Do(ctx, http.MethodGet, endpoint, nil, nil, DelegatedScopes, nil)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (s *Service) Put(ctx context.Context, remotePath string, content []byte) error {
	endpoint := "/me/drive/root:" + normalizeRemotePath(remotePath) + ":/content"
	_, _, err := s.client.Do(ctx, http.MethodPut, endpoint, nil, content, DelegatedScopes, nil)
	return err
}

func (s *Service) Mkdir(ctx context.Context, remotePath string) error {
	cleaned := normalizeRemotePath(remotePath)
	parent := path.Dir(cleaned)
	if parent == "." {
		parent = "/"
	}
	name := path.Base(cleaned)
	if name == "." || name == "/" || strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid folder path: %s", remotePath)
	}

	endpoint := "/me/drive/root/children"
	if parent != "/" {
		endpoint = "/me/drive/root:" + parent + ":/children"
	}

	payload := map[string]any{
		"name":                              name,
		"folder":                            map[string]any{},
		"@microsoft.graph.conflictBehavior": "rename",
	}

	_, _, err := s.client.Do(ctx, http.MethodPost, endpoint, nil, payload, DelegatedScopes, nil)
	return err
}

func (s *Service) Remove(ctx context.Context, remotePath string) error {
	endpoint := "/me/drive/root:" + normalizeRemotePath(remotePath)
	_, _, err := s.client.Do(ctx, http.MethodDelete, endpoint, nil, nil, DelegatedScopes, nil)
	return err
}

func normalizeRemotePath(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}

	trimmed = strings.TrimPrefix(trimmed, "~/")
	trimmed = strings.TrimPrefix(trimmed, "./")
	trimmed = strings.TrimPrefix(trimmed, "/")

	segments := strings.Split(trimmed, "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		escaped = append(escaped, url.PathEscape(segment))
	}

	if len(escaped) == 0 {
		return "/"
	}

	return "/" + filepath.ToSlash(strings.Join(escaped, "/"))
}
