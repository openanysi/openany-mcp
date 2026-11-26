package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func TestServerCard(t *testing.T) {
	if len(serverCardDescription) > 100 {
		t.Fatalf("description length %d exceeds Server Card maxLength 100", len(serverCardDescription))
	}
	handler := newServerCardHandler(t, Options{LiteEnabled: true})
	server := httptest.NewServer(handler)
	defer server.Close()

	get, body := doCard(t, server, http.MethodGet, "/server-card", nil)
	if get.StatusCode != http.StatusOK {
		t.Fatalf("GET status %d", get.StatusCode)
	}
	if got := get.Header.Get("Content-Type"); got != serverCardMediaType {
		t.Fatalf("Content-Type %q", got)
	}
	if strings.Contains(strings.ToLower(get.Header.Get("Content-Type")), "application/mcp-server+json") {
		t.Fatal("response used the deprecated server media type")
	}
	if get.Header.Get("Access-Control-Allow-Origin") != "*" ||
		get.Header.Get("Access-Control-Allow-Methods") != "GET" ||
		get.Header.Get("Access-Control-Allow-Headers") != "Content-Type, If-None-Match" ||
		get.Header.Get("Access-Control-Expose-Headers") != "ETag" {
		t.Fatalf("CORS headers: origin=%q methods=%q allow=%q expose=%q",
			get.Header.Get("Access-Control-Allow-Origin"),
			get.Header.Get("Access-Control-Allow-Methods"),
			get.Header.Get("Access-Control-Allow-Headers"),
			get.Header.Get("Access-Control-Expose-Headers"))
	}
	if get.Header.Get("Cache-Control") != "public, max-age=3600" {
		t.Fatalf("Cache-Control %q", get.Header.Get("Cache-Control"))
	}
	etag := get.Header.Get("ETag")
	if etag != serverCardETag || len(etag) < 3 || etag[0] != '"' || etag[len(etag)-1] != '"' {
		t.Fatalf("ETag %q", etag)
	}
	if get.Header.Get("Content-Length") != strconv.Itoa(len(body)) || string(body) != string(serverCardBody) {
		t.Fatalf("body mismatch length=%s bytes=%d", get.Header.Get("Content-Length"), len(body))
	}

	var card serverCardDocument
	if err := json.Unmarshal(body, &card); err != nil {
		t.Fatal(err)
	}
	if card.Schema != serverCardSchemaURL || card.Name != "si.openany/openany" || card.Title != "OpenAny" || card.Version != "0.1.0" {
		t.Fatalf("identity %+v", card)
	}
	if card.Version == "0.1.4" || strings.Contains(string(body), "unverified-draft") || strings.Contains(string(body), "si.openany/mcp") || strings.Contains(string(body), `"_meta"`) {
		t.Fatal("card published a rejected identity or draft warning")
	}
	if card.Description != serverCardDescription || card.WebsiteURL != "https://openany.si" {
		t.Fatalf("description/website %+v", card)
	}
	if card.Repository.URL != "https://github.com/openanysi/openany-mcp" || card.Repository.Source != "github" {
		t.Fatalf("repository %+v", card.Repository)
	}
	if len(card.Remotes) != 1 || card.Remotes[0].Type != "streamable-http" || card.Remotes[0].URL != "https://mcp.openany.si" {
		t.Fatalf("remote %+v", card.Remotes)
	}
	wantVersions := []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"}
	if strings.Join(card.Remotes[0].SupportedProtocolVersions, ",") != strings.Join(wantVersions, ",") {
		t.Fatalf("protocol versions %v", card.Remotes[0].SupportedProtocolVersions)
	}
	if len(card.Remotes[0].Headers) != 1 {
		t.Fatal("expected one header")
	}
	header := card.Remotes[0].Headers[0]
	if header.Name != "Authorization" || header.IsRequired || !header.IsSecret || header.Value != "Bearer {OPENANY_API_KEY}" {
		t.Fatalf("header %+v", header)
	}
	if !strings.Contains(header.Description, "do not authorize charges") || !strings.Contains(header.Description, "Bearer OpenAny API key") || !strings.Contains(header.Description, "not OAuth") {
		t.Fatalf("header description %q", header.Description)
	}
	variable, ok := header.Variables["OPENANY_API_KEY"]
	if !ok || variable.IsRequired || !variable.IsSecret || variable.Description != "OpenAny API key" {
		t.Fatalf("variable %+v", variable)
	}

	for _, match := range []string{etag, "W/" + etag, `"other", ` + etag, "*"} {
		res, cached := doCard(t, server, http.MethodGet, "/server-card", map[string]string{"If-None-Match": match})
		if res.StatusCode != http.StatusNotModified || len(cached) != 0 || res.Header.Get("ETag") != etag {
			t.Fatalf("If-None-Match %q: status=%d etag=%q body=%q", match, res.StatusCode, res.Header.Get("ETag"), cached)
		}
		if res.Header.Get("Cache-Control") != "public, max-age=3600" || res.Header.Get("Access-Control-Expose-Headers") != "ETag" {
			t.Fatalf("304 caching/CORS headers missing for %q", match)
		}
	}
	miss, missBody := doCard(t, server, http.MethodGet, "/server-card", map[string]string{"If-None-Match": `"not-the-card"`})
	if miss.StatusCode != http.StatusOK || string(missBody) != string(serverCardBody) {
		t.Fatalf("mismatched validator: status=%d bytes=%d", miss.StatusCode, len(missBody))
	}

	head, headBody := doCard(t, server, http.MethodHead, "/server-card", nil)
	if head.StatusCode != http.StatusOK || len(headBody) != 0 || head.Header.Get("Content-Length") != strconv.Itoa(len(serverCardBody)) || head.Header.Get("ETag") != etag || head.Header.Get("Content-Type") != serverCardMediaType {
		t.Fatalf("HEAD status=%d type=%q length=%q etag=%q body=%d", head.StatusCode, head.Header.Get("Content-Type"), head.Header.Get("Content-Length"), head.Header.Get("ETag"), len(headBody))
	}

	options, optionsBody := doCard(t, server, http.MethodOptions, "/server-card", nil)
	if options.StatusCode != http.StatusNoContent || len(optionsBody) != 0 || !strings.Contains(options.Header.Get("Allow"), "GET") || options.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("OPTIONS status=%d allow=%q origin=%q body=%q", options.StatusCode, options.Header.Get("Allow"), options.Header.Get("Access-Control-Allow-Origin"), optionsBody)
	}

	post, postBody := doCard(t, server, http.MethodPost, "/server-card", nil)
	if post.StatusCode != http.StatusMethodNotAllowed || strings.Contains(string(postBody), "si.openany/openany") || !strings.Contains(post.Header.Get("Allow"), "GET") {
		t.Fatalf("POST status=%d allow=%q body=%q", post.StatusCode, post.Header.Get("Allow"), postBody)
	}

	slash, _ := doCard(t, server, http.MethodGet, "/server-card/", nil)
	if slash.StatusCode != http.StatusNotFound {
		t.Fatalf("trailing slash status %d", slash.StatusCode)
	}

	mcp, _ := doCard(t, server, http.MethodGet, "/mcp", nil)
	if mcp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unready /mcp status %d; card route must not skip catalog readiness", mcp.StatusCode)
	}

	foreign := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/server-card", nil)
	foreign.Header.Set("Origin", "https://example.com")
	limited := newServerCardHandler(t, Options{LiteEnabled: true, Origins: []string{"https://openany.si"}})
	rec := httptest.NewRecorder()
	limited.ServeHTTP(rec, foreign)
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("foreign origin card status=%d acao=%q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
	health := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/healthz", nil)
	health.Header.Set("Origin", "https://example.com")
	healthRec := httptest.NewRecorder()
	limited.ServeHTTP(healthRec, health)
	if healthRec.Code != http.StatusForbidden {
		t.Fatalf("foreign origin health status %d", healthRec.Code)
	}

	denied := newServerCardHandler(t, Options{Hosts: []string{"mcp.openany.si"}})
	badHost := httptest.NewRequest(http.MethodGet, "http://evil.example/server-card", nil)
	badHost.Host = "evil.example"
	badRec := httptest.NewRecorder()
	denied.ServeHTTP(badRec, badHost)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("disallowed host status %d", badRec.Code)
	}
	goodHost := httptest.NewRequest(http.MethodGet, "http://mcp.openany.si/server-card", nil)
	goodHost.Host = "mcp.openany.si"
	goodRec := httptest.NewRecorder()
	denied.ServeHTTP(goodRec, goodHost)
	if goodRec.Code != http.StatusOK || goodRec.Header().Get("Content-Type") != serverCardMediaType {
		t.Fatalf("allowed host status=%d type=%q", goodRec.Code, goodRec.Header().Get("Content-Type"))
	}
}

func newServerCardHandler(t *testing.T, opts Options) *Handler {
	t.Helper()
	manager := catalog.NewManager(func(context.Context) ([]model.Record, error) { return nil, nil }, model.Overrides{Version: 1}, time.Minute, false)
	return New(manager, nil, opts)
}

func doCard(t *testing.T, server *httptest.Server, method, path string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, body
}
