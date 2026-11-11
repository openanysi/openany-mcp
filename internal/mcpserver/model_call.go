package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/schema"
)

func (h *Handler) executeModel(ctx context.Context, req *mcp.CallToolRequest, d *catalog.Discovery, api, version string, raw json.RawMessage, catalogHash string) gateway.Result {
	start := time.Now()
	if d.Kind(api) != "model" || d.Version(api) == "" {
		return gateway.Failure("openany_mcp_api_unavailable", "Model or API is not currently published; search again.", "validate", "not_started", nil, nil)
	}
	if d.Version(api) != version {
		return gateway.Failure("openany_mcp_api_changed", "Model definition or displayed price changed; call get_api again.", "validate", "not_started", nil, nil)
	}
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	args, err := catalog.ChatInputSchema().Validate(raw)
	if err != nil {
		result := gateway.Failure("openany_mcp_invalid_input", "Input parameters do not match the model chat schema.", "validate", "not_started", nil, nil)
		e := result.Value["error"].(map[string]any)
		e["details"] = schema.ValidationDetails(err)
		e["details_scope"] = "input"
		return result
	}
	if args["stream"] == true {
		return gateway.Failure("openany_mcp_invalid_input", "stream true is not supported for model calls. Omit stream or set it to false.", "validate", "not_started", nil, nil)
	}
	if supplied, ok := args["model"]; ok {
		value, _ := supplied.(string)
		if value != api {
			return gateway.Failure("openany_mcp_invalid_input", "input.model must equal the selected model_id.", "validate", "not_started", nil, nil)
		}
	} else {
		args["model"] = api
	}
	if h.models == nil {
		return gateway.Failure("openany_mcp_model_unavailable", "Model execution is not configured on this server.", "validate", "not_started", nil, nil)
	}
	if missing, blocked := missingCredentials(req); blocked {
		return missing
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
	result := clarifyRejectedKey(h.models.PostChatCompletion(ctx, args, headers))
	h.options.Logger.Info("model call completed", "model_id", api, "catalog_hash", catalogHash, "gateway_request_id", result.Value["request_id"], "http_status", result.Value["http_status"], "is_error", result.IsError, "duration_ms", time.Since(start).Milliseconds())
	return result
}
