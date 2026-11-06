package mcpserver

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/schema"
	"net/http"
	"time"
)

func (h *Handler) execute(ctx context.Context, req *mcp.CallToolRequest, b *catalog.Binding, name string, raw json.RawMessage, catalogHash string) (*mcp.CallToolResult, error) {
	return toMCP(h.executeResult(ctx, req, b, name, raw, catalogHash, false)), nil
}
func (h *Handler) executeResult(ctx context.Context, req *mcp.CallToolRequest, b *catalog.Binding, name string, raw json.RawMessage, catalogHash string, details bool) gateway.Result {
	start := time.Now()
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	args, err := b.Input.Validate(raw)
	if err != nil {
		result := gateway.Failure("openany_mcp_invalid_input", "Input parameters do not match the tool schema.", "validate", "not_started", nil, nil)
		if details {
			e := result.Value["error"].(map[string]any)
			e["details"] = schema.ValidationDetails(err)
			e["details_scope"] = "input"
		}
		return result
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return gateway.Failure("openany_mcp_busy", "MCP concurrency limit reached.", "admission", "not_started", nil, nil)
	}
	var headers http.Header
	if req.Extra != nil {
		headers = req.Extra.Header
	}
	result := h.gateway.Call(ctx, b.Request, args, headers)
	h.options.Logger.Info("tool completed", "api_id", b.APIID, "tool_name", name, "catalog_hash", catalogHash, "gateway_request_id", result.Value["request_id"], "http_status", result.Value["http_status"], "is_error", result.IsError, "duration_ms", time.Since(start).Milliseconds())
	return result
}
