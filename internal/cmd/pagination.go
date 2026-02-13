package cmd

import (
	"net/url"
	"strings"
)

var allowedGraphPageHosts = map[string]struct{}{
	"graph.microsoft.com":             {},
	"graph.microsoft.us":              {},
	"dod-graph.microsoft.us":          {},
	"graph.microsoft.de":              {},
	"microsoftgraph.chinacloudapi.cn": {},
}

func normalizePageToken(raw string) (string, error) {
	page := strings.TrimSpace(raw)
	if page == "" {
		return "", nil
	}

	u, err := url.Parse(page)
	if err != nil {
		return "", usage("--page must be an absolute http(s) URL")
	}
	if !u.IsAbs() || strings.TrimSpace(u.Host) == "" {
		return "", usage("--page must be an absolute http(s) URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", usage("--page must be an absolute http(s) URL")
	}
	if !isAllowedGraphPageHost(u.Hostname()) {
		return "", usage("--page URL must use a Microsoft Graph host")
	}

	return u.String(), nil
}

func isAllowedGraphPageHost(host string) bool {
	_, ok := allowedGraphPageHosts[strings.ToLower(strings.TrimSpace(host))]
	return ok
}
