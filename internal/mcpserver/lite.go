package mcpserver

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/gateway"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/schema"
)

func (h *Handler) newLiteServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "OpenAny Lite", Version: "0.1.0"}, &mcp.ServerOptions{Logger: h.options.Logger, Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{ListChanged: false}}, SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"}})
	stringInput := func(name string, required bool, max int) model.Input {
		raw, _ := json.Marshal(map[string]any{"type": "string", "minLength": 1, "maxLength": max})
		return model.Input{Name: name, Type: "string", Required: required, Schema: raw}
	}
	definitions := []struct {
		name, description string
		inputs            []model.Input
	}{
		{"search_apis", "Find OpenAny models and APIs by keywords and optional kind (api, model, or all; default all), provider, category, data_platform, data_product or capability. Model hits include model_id and, when the model catalog provides them, modalities, context length and a pricing summary. Returns bounded summaries and required inputs, not full schemas. Then call get_api. Keyword search supports limited bilingual aliases; it is not general semantic search.", []model.Input{stringInput("query", false, 256), stringInput("kind", false, 16), stringInput("provider", false, 64), stringInput("category", false, 128), stringInput("data_platform", false, 256), stringInput("data_product", false, 128), stringInput("capability", false, 256), {Name: "limit", Schema: json.RawMessage(`{"type":"integer","minimum":1,"maximum":10,"default":5}`)}, stringInput("cursor", false, 4096)}},
		{"get_api", "Read one model or API selected from search_apis. For an API, returns the exact input_schema, public examples, pricing and api_version. For a model, returns the chat completions input shape, model_id, and catalog pricing when the model catalog provides it. Read this before executing. Discovery does not run a paid call, and displayed pricing is not a price lock.", []model.Input{stringInput("api", true, 64)}},
		{"run_api", "Execute one selected OpenAny API, or send one selected model to chat completions, with the caller's current credentials. First call get_api. Pass its api and api_version, and input that matches input_schema. Model calls can incur charges. Supports finite JSON, UTF-8 plain text and Markdown. Optional output only changes presentation: raw (default), structural summary, or compact for JSON objects/arrays with explicit items_path for nested arrays. A timeout after dispatch has unknown execution or charging state: do not retry automatically.", []model.Input{stringInput("api", true, 64), {Name: "api_version", Required: true, Schema: json.RawMessage(`{"type":"string","pattern":"^[a-f0-9]{64}$"}`)}, {Name: "input", Required: true, Schema: json.RawMessage(`{"type":"object","additionalProperties":true}`)}, {Name: "output", Schema: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"mode":{"type":"string","enum":["raw","compact","summary"],"default":"raw"},"items_path":{"type":"string","maxLength":512,"description":"RFC6901 pointer relative to response data. Required to clip a nested array; empty selects a root array."},"fields":{"type":"array","minItems":1,"maxItems":20,"uniqueItems":true,"items":{"type":"string","minLength":1,"maxLength":128},"description":"Literal object keys at root object or selected array items. Does not change upstream fields or pricing."},"max_items":{"type":"integer","minimum":1,"maximum":100,"description":"Returned array limit, default 10 in compact mode. Requires root array or items_path."}}}`)}}},
	}
	for _, definition := range definitions {
		compiled, err := schema.Compile(model.Record{UnknownParameters: "reject", Inputs: definition.inputs})
		if err != nil {
			panic("invalid built-in MCP schema")
		}
		name := definition.name
		s.AddTool(&mcp.Tool{Name: name, Description: definition.description, InputSchema: compiled.Document}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			raw := req.Params.Arguments
			if len(raw) == 0 {
				raw = json.RawMessage(`{}`)
			}
			args, err := compiled.Validate(raw)
			if err != nil {
				result := gateway.Failure("openany_mcp_invalid_input", "Parameters do not match this discovery tool schema.", "validate", "not_started", nil, nil)
				result.Value["error"].(map[string]any)["details"] = schema.ValidationDetails(err)
				result.Value["error"].(map[string]any)["details_scope"] = "arguments"
				return toLiteMCP(result), nil
			}
			snapshot, _ := ctx.Value(snapshotKey{}).(*catalog.Snapshot)
			if snapshot == nil || snapshot.Discovery == nil {
				return toLiteMCP(gateway.Failure("openany_mcp_catalog_unavailable", "Discovery catalog is not ready.", "validate", "not_started", nil, nil)), nil
			}
			d := snapshot.Discovery
			text := func(key string) string { x, _ := args[key].(string); return x }
			switch name {
			case "search_apis":
				limit := 0
				if number, ok := args["limit"].(json.Number); ok {
					n, err := number.Float64()
					if err != nil {
						return toLiteMCP(gateway.Failure("openany_mcp_invalid_input", "Invalid search limit.", "validate", "not_started", nil, nil)), nil
					}
					limit = int(n)
				}
				result, err := d.SearchFiltered(catalog.SearchQuery{Query: text("query"), Kind: text("kind"), Provider: text("provider"), Category: text("category"), Platform: text("data_platform"), Product: text("data_product"), Capability: text("capability"), Limit: limit, Cursor: text("cursor")})
				if err != nil {
					return toLiteMCP(gateway.Failure("openany_mcp_invalid_search", err.Error(), "validate", "not_started", nil, nil)), nil
				}
				return toLiteMCP(gateway.Result{Value: map[string]any{"results": result.Results, "total": result.Total, "ranking": result.Ranking, "next_cursor": result.NextCursor, "metadata_available": d.MetadataAvailable, "models_available": d.ModelsAvailable, "models_stale": d.ModelsStale}}), nil
			case "get_api":
				detail, ok := d.Detail(text("api"))
				if !ok {
					return unavailableAPI(), nil
				}
				return toLiteMCP(gateway.Result{Value: detail}), nil
			case "run_api":
				api := text("api")
				if d.Kind(api) == "model" {
					input, err := json.Marshal(args["input"])
					if err != nil {
						return toLiteMCP(gateway.Failure("openany_mcp_invalid_input", "Invalid input object.", "validate", "not_started", nil, nil)), nil
					}
					output, err := parseOutput(args["output"])
					if err != nil {
						return toLiteMCP(gateway.Failure("openany_mcp_invalid_output", err.Error(), "validate", "not_started", nil, nil)), nil
					}
					result := withRecovery(present(h.executeModel(ctx, req, d, api, text("api_version"), input, snapshot.Hash), output))
					return toMCP(h.gateway.LimitResult(result)), nil
				}
				binding := snapshot.Tools[api]
				if binding == nil || d.Version(api) == "" {
					return unavailableAPI(), nil
				}
				if d.Version(api) != text("api_version") {
					return toLiteMCP(gateway.Failure("openany_mcp_api_changed", "API definition or displayed price changed; call get_api again.", "validate", "not_started", nil, nil)), nil
				}
				input, err := json.Marshal(args["input"])
				if err != nil {
					return toLiteMCP(gateway.Failure("openany_mcp_invalid_input", "Invalid input object.", "validate", "not_started", nil, nil)), nil
				}
				output, err := parseOutput(args["output"])
				if err != nil {
					return toLiteMCP(gateway.Failure("openany_mcp_invalid_output", err.Error(), "validate", "not_started", nil, nil)), nil
				}
				result := withRecovery(present(h.executeResult(ctx, req, binding, api, input, snapshot.Hash, true), output))
				return toMCP(h.gateway.LimitResult(result)), nil
			}
			return unavailableAPI(), nil
		})
	}
	return s
}
func unavailableAPI() *mcp.CallToolResult {
	return toLiteMCP(gateway.Failure("openany_mcp_api_unavailable", "Model or API is not currently published; search again.", "validate", "not_started", nil, nil))
}

func toLiteMCP(result gateway.Result) *mcp.CallToolResult { return toMCP(withRecovery(result)) }
