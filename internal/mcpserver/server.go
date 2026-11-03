package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

type Options struct {
	MaxBody  int64
	MaxCalls int
	Hosts    []string
	Origins  []string
	Logger   *slog.Logger
}
type serverKey struct{}
type viewSet struct {
	snapshot *catalog.Snapshot
	servers  map[string]*mcp.Server
}
type Handler struct {
	manager *catalog.Manager
	gateway *gateway.Client
	options Options
	slots   chan struct{}
	mu      sync.Mutex
	views   *viewSet
	sdk     *mcp.StreamableHTTPHandler
}

func New(manager *catalog.Manager, client *gateway.Client, opts Options) *Handler {
	if opts.MaxCalls < 1 {
		opts.MaxCalls = 16
	}
	if opts.MaxBody < 1 {
		opts.MaxBody = 8 << 20
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	h := &Handler{manager: manager, gateway: client, options: opts, slots: make(chan struct{}, opts.MaxCalls)}
	h.sdk = mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { s, _ := r.Context().Value(serverKey{}).(*mcp.Server); return s }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: opts.MaxBody, Logger: opts.Logger, DisableLocalhostProtection: true})
	return h
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.allowedHost(r.Host) {
		http.Error(w, "Forbidden host", 403)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && !slices.Contains(h.options.Origins, origin) {
		http.Error(w, "Forbidden origin", 403)
		return
	}
	if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/readyz" {
			if !h.manager.Ready() {
				w.WriteHeader(503)
			}
			json.NewEncoder(w).Encode(h.manager.Status())
		} else {
			w.Write([]byte(`{"ok":true}`))
		}
		return
	}
	scope := ""
	if r.URL.Path != "/mcp" {
		if !strings.HasPrefix(r.URL.Path, "/mcp/") || strings.Contains(r.URL.Path[len("/mcp/"):], "/") {
			http.NotFound(w, r)
			return
		}
		scope = strings.TrimPrefix(r.URL.Path, "/mcp/")
		if scope == "" {
			http.NotFound(w, r)
			return
		}
	}
	if !h.manager.Ready() {
		http.Error(w, "Catalog unavailable", 503)
		return
	}
	snapshot := h.manager.Current()
	if scope != "" && !snapshot.Providers[scope] {
		http.NotFound(w, r)
		return
	}
	server := h.server(snapshot, scope)
	h.sdk.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), serverKey{}, server)))
}
func (h *Handler) allowedHost(host string) bool {
	if len(h.options.Hosts) > 0 {
		return slices.Contains(h.options.Hosts, host)
	}
	name, _, err := net.SplitHostPort(host)
	if err != nil {
		name = host
	}
	ip := net.ParseIP(name)
	return name == "localhost" || (ip != nil && ip.IsLoopback())
}
func (h *Handler) server(snapshot *catalog.Snapshot, scope string) *mcp.Server {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.views == nil || h.views.snapshot != snapshot {
		h.views = &viewSet{snapshot: snapshot, servers: map[string]*mcp.Server{}}
	}
	if s := h.views.servers[scope]; s != nil {
		return s
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "OpenAny", Version: "0.1.0"}, &mcp.ServerOptions{Logger: h.options.Logger, Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{ListChanged: false}}, SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"}, PageSize: 50})
	tools := []*mcp.Tool{}
	for _, name := range snapshot.Names {
		b := snapshot.Tools[name]
		if scope != "" && b.Record.ProviderID != scope {
			continue
		}
		tool := &mcp.Tool{Name: name, Title: b.Record.Name, Description: b.Description, InputSchema: b.Input.Document}
		tools = append(tools, tool)
		s.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			start := time.Now()
			raw := req.Params.Arguments
			if len(raw) == 0 {
				raw = json.RawMessage(`{}`)
			}
			args, err := b.Input.Validate(raw)
			if err != nil {
				return toMCP(gateway.Failure("openany_mcp_invalid_input", "Input parameters do not match the tool schema.", "validate", "not_started", nil, nil)), nil
			}
			select {
			case h.slots <- struct{}{}:
				defer func() { <-h.slots }()
			default:
				return toMCP(gateway.Failure("openany_mcp_busy", "MCP concurrency limit reached.", "admission", "not_started", nil, nil)), nil
			}
			var headers http.Header
			if req.Extra != nil {
				headers = req.Extra.Header
			}
			result := h.gateway.Call(ctx, b.Record, b.Override, args, headers)
			h.options.Logger.Info("tool completed", "api_id", b.Record.APIID, "tool_name", name, "catalog_hash", snapshot.Hash, "gateway_request_id", result.Value["request_id"], "http_status", result.Value["http_status"], "is_error", result.IsError, "duration_ms", time.Since(start).Milliseconds())
			return toMCP(result), nil
		})
	}
	s.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/list" {
				return next(ctx, method, req)
			}
			list, ok := req.(*mcp.ListToolsRequest)
			if !ok {
				return nil, fmt.Errorf("invalid list request")
			}
			cursor := ""
			if list.Params != nil {
				cursor = list.Params.Cursor
			}
			return page(tools, snapshot.Hash, scope, cursor)
		}
	})
	h.views.servers[scope] = s
	return s
}
func toMCP(r gateway.Result) *mcp.CallToolResult {
	raw, _ := json.Marshal(r.Value)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: r.Value, IsError: r.IsError}
}

type cursorValue struct {
	Version int    `json:"v"`
	Hash    string `json:"hash"`
	Scope   string `json:"scope"`
	After   string `json:"after"`
}

func page(tools []*mcp.Tool, hash, scope, cursor string) (*mcp.ListToolsResult, error) {
	start := 0
	if cursor != "" {
		invalid := func() (*mcp.ListToolsResult, error) {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "Invalid or outdated cursor; restart tools/list without a cursor."}
		}
		if len(cursor) > 2048 {
			return invalid()
		}
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return invalid()
		}
		var c cursorValue
		if json.Unmarshal(raw, &c) != nil || c.Version != 1 || c.Hash != hash || c.Scope != scope {
			return invalid()
		}
		found := false
		for i, t := range tools {
			if t.Name == c.After {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return invalid()
		}
	}
	end := start + 50
	if end > len(tools) {
		end = len(tools)
	}
	result := &mcp.ListToolsResult{Tools: tools[start:end]}
	if end < len(tools) {
		raw, _ := json.Marshal(cursorValue{1, hash, scope, tools[end-1].Name})
		result.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return result, nil
}
