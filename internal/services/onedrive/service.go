package onedrive

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/jaredpalmer/mogcli/internal/graph"
)

var listOneDriveScopes = []string{"Files.Read"}
var getOneDriveScopes = []string{"Files.Read"}
var putOneDriveScopes = []string{"Files.ReadWrite"}
var mkdirOneDriveScopes = []string{"Files.ReadWrite"}
var removeOneDriveScopes = []string{"Files.ReadWrite"}

type Service struct {
	client      *graph.Client
	appOnlyUser string
}

func New(client *graph.Client, appOnlyUser string) *Service {
	return &Service{client: client, appOnlyUser: strings.TrimSpace(appOnlyUser)}
}

func (s *Service) List(ctx context.Context, remotePath string, max int, page string) ([]map[string]any, string, error) {
	if strings.TrimSpace(page) != "" {
		return s.client.Paginate(ctx, strings.TrimSpace(page), nil, listOneDriveScopes, max)
	}

	endpoint := s.driveEndpoint() + "/root/children"
	if cleaned := normalizeRemotePath(remotePath); cleaned != "/" {
		endpoint = s.driveEndpoint() + "/root:" + cleaned + ":/children"
	}

	query := url.Values{}
	if max > 0 {
		query.Set("$top", fmt.Sprintf("%d", max))
	}

	return s.client.Paginate(ctx, endpoint, query, listOneDriveScopes, max)
}

func (s *Service) Get(ctx context.Context, remotePath string) ([]byte, error) {
	endpoint := s.driveEndpoint() + "/root:" + normalizeRemotePath(remotePath) + ":/content"
	_, body, err := s.client.Do(ctx, http.MethodGet, endpoint, nil, nil, getOneDriveScopes, nil)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (s *Service) Put(ctx context.Context, remotePath string, content []byte) error {
	endpoint := s.driveEndpoint() + "/root:" + normalizeRemotePath(remotePath) + ":/content"
	_, _, err := s.client.Do(ctx, http.MethodPut, endpoint, nil, content, putOneDriveScopes, nil)
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

	endpoint := s.driveEndpoint() + "/root/children"
	if parent != "/" {
		endpoint = s.driveEndpoint() + "/root:" + parent + ":/children"
	}

	payload := map[string]any{
		"name":                              name,
		"folder":                            map[string]any{},
		"@microsoft.graph.conflictBehavior": "rename",
	}

	_, _, err := s.client.Do(ctx, http.MethodPost, endpoint, nil, payload, mkdirOneDriveScopes, nil)
	return err
}

func (s *Service) Remove(ctx context.Context, remotePath string) error {
	endpoint := s.driveEndpoint() + "/root:" + normalizeRemotePath(remotePath)
	_, _, err := s.client.Do(ctx, http.MethodDelete, endpoint, nil, nil, removeOneDriveScopes, nil)
	return err
}

func (s *Service) driveEndpoint() string {
	if s.appOnlyUser != "" {
		return "/users/" + url.PathEscape(s.appOnlyUser) + "/drive"
	}
	return "/me/drive"
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
