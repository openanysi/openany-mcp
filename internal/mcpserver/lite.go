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
		{"search_apis", "Find OpenAny APIs by keywords and optional provider. Returns a few summaries, not schemas. Start here, then call get_api for the selected API. Keyword search supports limited bilingual aliases; it is not general semantic search.", []model.Input{stringInput("query", false, 256), stringInput("provider", false, 64), {Name: "limit", Schema: json.RawMessage(`{"type":"integer","minimum":1,"maximum":10,"default":5}`)}, stringInput("cursor", false, 2048)}},
		{"get_api", "Read one API's exact input_schema, public pricing, limitations and api_version. Use the api identifier returned by search_apis. Read this before executing. Discovery does not execute a paid API; displayed pricing is not a price lock.", []model.Input{stringInput("api", true, 64)}},
		{"run_api", "Execute one selected OpenAny API using the caller's current gateway credentials. First call get_api. Pass its api and api_version, with input matching input_schema. This can incur charges and side effects. A timeout after dispatch has unknown execution/charging state: do not retry automatically.", []model.Input{stringInput("api", true, 64), {Name: "api_version", Required: true, Schema: json.RawMessage(`{"type":"string","pattern":"^[a-f0-9]{64}$"}`)}, {Name: "input", Required: true, Schema: json.RawMessage(`{"type":"object","additionalProperties":true}`)}}},
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
				return toMCP(gateway.Failure("openany_mcp_invalid_input", "Parameters do not match this discovery tool schema.", "validate", "not_started", nil, nil)), nil
			}
			snapshot, _ := ctx.Value(snapshotKey{}).(*catalog.Snapshot)
			if snapshot == nil || snapshot.Discovery == nil {
				return toMCP(gateway.Failure("openany_mcp_catalog_unavailable", "Discovery catalog is not ready.", "validate", "not_started", nil, nil)), nil
			}
			d := snapshot.Discovery
			text := func(key string) string { x, _ := args[key].(string); return x }
			switch name {
			case "search_apis":
				limit := 0
				if number, ok := args["limit"].(json.Number); ok {
					n, _ := number.Int64()
					limit = int(n)
				}
				result, err := d.Search(text("query"), text("provider"), limit, text("cursor"))
				if err != nil {
					return toMCP(gateway.Failure("openany_mcp_invalid_search", err.Error(), "validate", "not_started", nil, nil)), nil
				}
				return toMCP(gateway.Result{Value: map[string]any{"results": result.Results, "total": result.Total, "ranking": result.Ranking, "next_cursor": result.NextCursor}}), nil
			case "get_api":
				detail, ok := d.Detail(text("api"))
				if !ok {
					return unavailableAPI(), nil
				}
				return toMCP(gateway.Result{Value: detail}), nil
			case "run_api":
				api := text("api")
				binding := snapshot.Tools[api]
				if binding == nil {
					return unavailableAPI(), nil
				}
				if d.Version(api) != text("api_version") {
					return toMCP(gateway.Failure("openany_mcp_api_changed", "API definition or displayed price changed; call get_api again.", "validate", "not_started", nil, nil)), nil
				}
				input, err := json.Marshal(args["input"])
				if err != nil {
					return toMCP(gateway.Failure("openany_mcp_invalid_input", "Invalid input object.", "validate", "not_started", nil, nil)), nil
				}
				return h.execute(ctx, req, binding, api, input, snapshot.Hash)
			}
			return unavailableAPI(), nil
		})
	}
	return s
}
func unavailableAPI() *mcp.CallToolResult {
	return toMCP(gateway.Failure("openany_mcp_api_unavailable", "API is not currently published; search again.", "validate", "not_started", nil, nil))
}
