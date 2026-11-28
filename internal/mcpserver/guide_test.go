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
	if status != 200 || !strings.HasPrefix(media, "text/plain") || !strings.HasPrefix(body, "# OpenAny MCP\n") || !strings.Contains(body, "Get started in 3 steps") || !strings.Contains(body, "Choose an endpoint") || strings.Contains(body, "选择入口") || strings.Contains(body, "developer-docs") {
		t.Fatal("default guide", status, media, body[:80])
	}
	for _, phrase := range []string{"One MCP endpoint for keyword research, Xiaohongshu and Douyin posts", "Xiaohongshu / 小红书 / RedNote", "Douyin / 抖音 / TikTok China", "Weibo / 微博", "international TikTok", "Is this keyword worth writing?"} {
		if !strings.Contains(body, phrase) {
			t.Fatalf("english guide missing %q", phrase)
		}
	}
	status, media, body = get("/", "*/*", "zh-CN,zh;q=0.9,en;q=0.8")
	if status != 200 || !strings.HasPrefix(media, "text/plain") || !strings.Contains(body, "选择入口") || strings.Contains(body, "Choose an endpoint") {
		t.Fatal("accept-language zh", media, strings.Contains(body, "选择入口"))
	}
	if !strings.Contains(body, "一个入口，让 agent 去查搜索词、小红书或抖音、找邮箱、看币价、调用模型。浏览和查价免费，执行按次付费.") || !strings.Contains(body, "抖音 / Douyin / TikTok China") || !strings.Contains(body, "这个关键词在 Google 上值得写吗？") {
		t.Fatal("chinese guide hook")
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
	if status != 200 || !strings.HasPrefix(media, "text/html") || !strings.Contains(body, "<h1>") || !strings.Contains(body, "<table>") || !strings.Contains(body, "<pre><code>") || !strings.Contains(body, "https://mcp.openany.si") || !strings.Contains(body, "<details open><summary>Developer docs</summary>") || strings.Count(body, "<script>") != 1 || strings.Contains(body, "选择入口") {
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

func TestGuidePresentationDoesNotCallGatewayOrLeakMarkers(t *testing.T) {
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) {
		t.Fatal("guide must not load a live catalog")
		return nil, nil
	}, model.Overrides{Version: 1}, time.Minute, false)
	// A nil gateway also makes an accidental execution path fail this test.
	handler := New(manager, nil, Options{LiteEnabled: true})
	for _, path := range []string{"/", "/docs/mcp", "/?lang=zh", "/docs/mcp?lang=zh"} {
		for _, accept := range []string{"text/plain", "text/html"} {
			req := httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil)
			req.Header.Set("Accept", accept)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			body := res.Body.String()
			if res.Code != 200 || strings.Contains(body, "<!--") || !strings.Contains(body, "run_api") || !strings.Contains(body, "MCP_PUBLIC_HOSTS") {
				t.Fatal(path, accept, "guide content or routing changed")
			}
		}
		req := httptest.NewRequest(http.MethodHead, "http://localhost"+path, nil)
		req.Header.Set("Accept", "text/html")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != 200 || res.Body.Len() != 0 {
			t.Fatal("HEAD must remain bodyless", path, res.Code)
		}
	}
}
