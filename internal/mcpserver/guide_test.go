package mcpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func TestGuideLanguageAndHTMLNegotiation(t *testing.T) {
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false)
	server := httptest.NewServer(New(manager, nil, Options{LiteEnabled: true}))
	defer server.Close()
	get := func(path, accept, language string) (int, string, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		if language != "" {
			req.Header.Set("Accept-Language", language)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Content-Type"), string(body)
	}
	status, media, body := get("/", "", "")
	if status != 200 || !strings.HasPrefix(media, "text/plain") || !strings.HasPrefix(body, "# OpenAny MCP\n") || !strings.Contains(body, "Choose an endpoint") || strings.Contains(body, "选择入口") {
		t.Fatal("default guide", status, media, body[:80])
	}
	status, media, body = get("/", "*/*", "zh-CN,zh;q=0.9,en;q=0.8")
	if status != 200 || !strings.HasPrefix(media, "text/plain") || !strings.Contains(body, "选择入口") || strings.Contains(body, "Choose an endpoint") {
		t.Fatal("accept-language zh", media, strings.Contains(body, "选择入口"))
	}
	status, media, body = get("/?lang=en", "", "zh-CN,zh;q=0.9")
	if !strings.HasPrefix(media, "text/plain") || !strings.Contains(body, "Choose an endpoint") {
		t.Fatal("lang=en did not override", media)
	}
	status, media, body = get("/docs/mcp?lang=zh", "text/plain", "en-US,en;q=0.9")
	if !strings.HasPrefix(media, "text/plain") || !strings.Contains(body, "选择入口") {
		t.Fatal("lang=zh did not override", media)
	}
	status, media, body = get("/", "text/html,application/xhtml+xml", "en-US")
	if status != 200 || !strings.HasPrefix(media, "text/html") || !strings.Contains(body, "<h1>") || !strings.Contains(body, "<table>") || !strings.Contains(body, "<pre><code>") || !strings.Contains(body, "https://mcp.openany.si") || strings.Contains(body, "<script") || strings.Contains(body, "选择入口") {
		t.Fatal("english html", status, media)
	}
	status, media, body = get("/docs/mcp", "text/html", "zh")
	if !strings.HasPrefix(media, "text/html") || !strings.Contains(body, "lang=\"zh\"") || !strings.Contains(body, "选择入口") || !strings.Contains(body, "<table>") {
		t.Fatal("chinese html", media)
	}
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/html, application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if strings.Contains(res.Header.Get("Content-Type"), "text/html") || strings.Contains(string(raw), "<h1>") || strings.Contains(string(raw), "Choose an endpoint") {
		t.Fatal("POST / returned the browser guide", res.StatusCode, res.Header.Get("Content-Type"), string(raw))
	}
}
