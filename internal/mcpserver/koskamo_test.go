package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func TestKoskamoClaimFile(t *testing.T) {
	const want = "koskamo-claim=fBvJozPROrZ9fufMAMhI1e3w\n"
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false)
	server := httptest.NewServer(New(manager, nil, Options{}))
	defer server.Close()
	base, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	fetch := func(method, path string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		if res.Header.Get("Location") != "" || res.Request.URL.Host != base.Host {
			t.Fatal("claim response changed host", method, res.StatusCode, res.Header.Get("Location"), res.Request.URL.Host)
		}
		return res
	}
	get := fetch(http.MethodGet, koskamoClaimPath)
	body, _ := io.ReadAll(get.Body)
	if get.StatusCode != http.StatusOK || get.Header.Get("Content-Type") != "text/plain; charset=utf-8" || string(body) != want || get.Header.Get("Content-Length") != "39" {
		t.Fatalf("GET claim: status=%d type=%q length=%q body=%q", get.StatusCode, get.Header.Get("Content-Type"), get.Header.Get("Content-Length"), body)
	}
	head := fetch(http.MethodHead, koskamoClaimPath)
	headBody, _ := io.ReadAll(head.Body)
	if head.StatusCode != http.StatusOK || head.Header.Get("Content-Type") != "text/plain; charset=utf-8" || len(headBody) != 0 || head.Header.Get("Content-Length") != "39" {
		t.Fatalf("HEAD claim: status=%d type=%q length=%q body=%q", head.StatusCode, head.Header.Get("Content-Type"), head.Header.Get("Content-Length"), headBody)
	}
	post := fetch(http.MethodPost, koskamoClaimPath)
	post.Body.Close()
	if post.StatusCode != http.StatusMethodNotAllowed {
		t.Fatal("claim path accepted POST", post.StatusCode)
	}
	other := fetch(http.MethodGet, koskamoClaimPath+"/")
	other.Body.Close()
	if other.StatusCode != http.StatusNotFound {
		t.Fatal("trailing slash matched the claim path", other.StatusCode)
	}
}
