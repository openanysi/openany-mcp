package mcpserver

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"strconv"
)

func (h *Handler) withAlternatives(result gateway.Result, discovery *catalog.Discovery) gateway.Result {
	e, ok := result.Value["error"].(map[string]any)
	if !ok {
		return result
	}
	values, ok := e["alternatives"].([]any)
	if !ok {
		return result
	}
	if !h.options.AlternativesEnabled {
		delete(e, "alternatives")
		delete(e, "alternatives_version")
		if recovery, ok := e["recovery"].(map[string]any); ok {
			recovery["primary_action"] = "check_original_request"
			recovery["next_step_target"] = "https://openany.si/dashboard?page=usage"
			e["message"] = "Check the original request and charging state before continuing."
		}
		return result
	}
	for _, raw := range values {
		c, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		var api string
		if c["kind"] == "api" && discovery != nil {
			id, _ := c["api_id"].(string)
			n, _ := c["api_revision"].(json.Number)
			revision, _ := strconv.Atoi(string(n))
			api, _ = discovery.ResolveRecoveryAPI(id, revision)
		} else if c["kind"] == "model" && discovery != nil {
			execution, _ := c["execution"].(map[string]any)
			overrides, _ := execution["request_overrides"].(map[string]any)
			id, _ := c["model_id"].(string)
			if execution["protocol"] == "openai-chat-completions" && execution["path"] == "/v1/chat/completions" && overrides["stream"] != true && len(id) <= 64 && discovery.Kind(id) == "model" && discovery.Version(id) != "" {
				api = id
			}
		}
		if api != "" && h.options.LiteEnabled {
			c["continuation"] = map[string]any{"mcp": map[string]any{"endpoint": "/mcp-lite", "connect_before_use": true, "inspect": map[string]any{"tool": "get_api", "arguments": map[string]any{"api": api}}, "execute_tool": "run_api", "refresh_api_version": true}}
		}
	}
	return result
}
