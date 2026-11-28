package seo

import (
	"fmt"
	"strings"
)

// NormalizeTarget turns a domain, subdomain, or page into the form DataForSEO
// Ranked Keywords and Keywords For Site require.
// A bare domain drops https:// and www. A page URL keeps a path and uses https://.
func NormalizeTarget(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("target is empty")
	}
	if strings.ContainsAny(s, " \t\r\n") || strings.Contains(s, "@") || strings.Contains(s, "\\") {
		return "", fmt.Errorf("target must be a domain or a page URL")
	}
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "https://"):
		s = s[len("https://"):]
	case strings.HasPrefix(lower, "http://"):
		s = s[len("http://"):]
	}
	s = strings.TrimPrefix(s, "//")
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "/") {
		return "", fmt.Errorf("target must be a domain or a page URL")
	}
	host, rest, _ := strings.Cut(s, "/")
	if host == "" || strings.Contains(host, "..") {
		return "", fmt.Errorf("target must be a domain or a page URL")
	}
	page := ""
	if rest != "" || strings.Contains(s, "?") || strings.Contains(s, "#") {
		page = strings.TrimPrefix(s[len(host):], "/")
		page = strings.Trim(page, "/")
	}
	host = strings.TrimSuffix(host, ".")
	bareHost := host
	if strings.HasPrefix(strings.ToLower(bareHost), "www.") {
		bareHost = bareHost[len("www."):]
	}
	if bareHost == "" || !strings.Contains(bareHost, ".") || strings.Contains(bareHost, "?") || strings.Contains(bareHost, "#") {
		return "", fmt.Errorf("target must be a domain or a page URL")
	}
	if page == "" {
		return strings.ToLower(bareHost), nil
	}
	return "https://" + host + "/" + page, nil
}
