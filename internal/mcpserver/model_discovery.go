package mcpserver

import (
	"context"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
)

// Unpublished models remain non-executable. This path only supplies current
// catalog hints and a get_api continuation; it never calls a business endpoint.
func (h *Handler) unavailableModel(ctx context.Context, d *catalog.Discovery, id string) gateway.Result {
	result := gateway.Failure("openany_mcp_api_unavailable", "Model or API is not currently published; search again.", "validate", "not_started", nil, nil)
	if !h.options.AlternativesEnabled || !d.ModelsAvailable || d.ModelsStale || h.models == nil {
		return result
	}
	discovery, err := h.models.ModelRecovery(ctx, id)
	if err != nil || discovery.SourceState != "unavailable" {
		return result
	}
	var related []any
	for _, model := range discovery.RelatedModels {
		if d.Kind(model.ModelID) != "model" || d.Version(model.ModelID) == "" || !d.SupportsChatDiscovery(model.ModelID) {
			continue
		}
		related = append(related, map[string]any{
			"model_id": model.ModelID, "provider_id": model.ProviderID, "relation": model.Relation, "protocol": model.Protocol, "pricing": model.Pricing,
			"continuation": map[string]any{"mcp": map[string]any{"endpoint": "/mcp-lite", "connect_before_use": true, "inspect": map[string]any{"tool": "get_api", "arguments": map[string]any{"api": model.ModelID}}}},
		})
	}
	if len(related) > 0 {
		e := result.Value["error"].(map[string]any)
		e["related_models"] = related
		e["charge_state"] = "not_charged"
		e["message"] = "The original model has no available Provider. Related models support the same chat modality; inspect a selected model with get_api before deciding whether to use it."
	}
	return h.gateway.LimitResult(result)
}
