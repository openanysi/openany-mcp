package mcpserver

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/openany/openany-mcp/internal/gateway"
)

type outputOptions struct {
	Mode      string
	ItemsPath *string
	Fields    []string
	MaxItems  int
}

func parseOutput(value any) (outputOptions, error) {
	o := outputOptions{Mode: "raw"}
	if value == nil {
		return o, nil
	}
	m, ok := value.(map[string]any)
	if !ok {
		return o, fmt.Errorf("output must be an object")
	}
	for key := range m {
		if key != "mode" && key != "items_path" && key != "fields" && key != "max_items" {
			return o, fmt.Errorf("unknown output option")
		}
	}
	if v, exists := m["mode"]; exists {
		o.Mode, ok = v.(string)
		if !ok {
			return o, fmt.Errorf("output.mode must be a string")
		}
	}
	if o.Mode != "raw" && o.Mode != "compact" && o.Mode != "summary" {
		return o, fmt.Errorf("output.mode must be raw, compact or summary")
	}
	if p, exists := m["items_path"]; exists {
		s, ok := p.(string)
		if !ok || len(s) > 512 || !validPointer(s) {
			return o, fmt.Errorf("output.items_path must be a JSON pointer relative to data")
		}
		o.ItemsPath = &s
	}
	if fields, exists := m["fields"]; exists {
		a, ok := fields.([]any)
		if !ok || len(a) < 1 || len(a) > 20 {
			return o, fmt.Errorf("output.fields must contain 1 to 20 object keys")
		}
		for _, v := range a {
			s, ok := v.(string)
			if !ok || s == "" || len(s) > 128 || strings.IndexFunc(s, unicode.IsControl) >= 0 {
				return o, fmt.Errorf("invalid output.fields key")
			}
			o.Fields = append(o.Fields, s)
		}
	}
	if v, exists := m["max_items"]; exists {
		n, ok := v.(json.Number)
		if !ok {
			return o, fmt.Errorf("invalid output.max_items")
		}
		f, err := n.Float64()
		if err != nil || f < 1 || f > 100 || f != float64(int(f)) {
			return o, fmt.Errorf("output.max_items must be an integer from 1 to 100")
		}
		o.MaxItems = int(f)
	}
	if o.Mode != "compact" && (o.ItemsPath != nil || len(o.Fields) > 0 || o.MaxItems != 0) {
		return o, fmt.Errorf("items_path, fields and max_items require compact mode")
	}
	return o, nil
}
func validPointer(p string) bool {
	if p == "" {
		return true
	}
	return strings.HasPrefix(p, "/") && !strings.Contains(strings.ReplaceAll(strings.ReplaceAll(p, "~0", ""), "~1", ""), "~") && strings.IndexFunc(p, unicode.IsControl) < 0
}
func lookupOutput(v any, path string) (any, bool) {
	if path == "" {
		return v, true
	}
	for _, part := range strings.Split(path[1:], "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch x := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = x[part]
			if !ok {
				return nil, false
			}
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(x) || (len(part) > 1 && part[0] == '0') {
				return nil, false
			}
			v = x[i]
		default:
			return nil, false
		}
	}
	return v, true
}

// structuralSummary contains no scalar response values and assigns no business success.
func structuralSummary(data any) map[string]any {
	out := map[string]any{"business_status": "not_interpreted"}
	switch v := data.(type) {
	case map[string]any:
		out["type"] = "object"
		keys := []string{}
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out["field_count"] = len(keys)
		if len(keys) > 12 {
			keys = keys[:12]
		}
		safe := []string{}
		for _, k := range keys {
			if len(k) > 80 || strings.IndexFunc(k, unicode.IsControl) >= 0 {
				k = "[field]"
			}
			safe = append(safe, k)
		}
		out["fields"] = safe
	case []any:
		out["type"] = "array"
		out["returned_items"] = len(v)
	case nil:
		out["type"] = "null"
	case string:
		out["type"] = "string"
		out["characters"] = utf8.RuneCountInString(v)
	case bool:
		out["type"] = "boolean"
	default:
		out["type"] = "number"
	}
	return out
}
func projectTarget(v any, o outputOptions) (any, bool, bool) {
	removed := false
	fields := func(v any) (any, bool) {
		if len(o.Fields) == 0 {
			return v, true
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		copy := map[string]any{}
		for _, k := range o.Fields {
			if value, exists := m[k]; exists {
				copy[k] = value
			}
		}
		if len(copy) < len(m) {
			removed = true
		}
		return copy, true
	}
	if a, ok := v.([]any); ok {
		n := len(a)
		limit := o.MaxItems
		if limit == 0 {
			limit = 10
		}
		if n > limit {
			n = limit
			removed = true
		}
		out := []any{}
		for _, item := range a[:n] {
			projected, ok := fields(item)
			if !ok {
				return nil, false, false
			}
			out = append(out, projected)
		}
		return out, removed, true
	}
	if o.ItemsPath != nil || o.MaxItems != 0 {
		return nil, false, false
	} // A row path/limit requires an actual array.
	result, ok := fields(v)
	return result, removed, ok
}
func replaceOutput(v any, path string, replacement any) any {
	if path == "" {
		return replacement
	}
	parts := strings.SplitN(path[1:], "/", 2)
	key := strings.ReplaceAll(strings.ReplaceAll(parts[0], "~1", "/"), "~0", "~")
	rest := ""
	if len(parts) == 2 {
		rest = "/" + parts[1]
	}
	switch x := v.(type) {
	case map[string]any:
		copy := map[string]any{}
		for k, v := range x {
			copy[k] = v
		}
		copy[key] = replaceOutput(x[key], rest, replacement)
		return copy
	case []any:
		i, _ := strconv.Atoi(key)
		copy := append([]any{}, x...)
		copy[i] = replaceOutput(x[i], rest, replacement)
		return copy
	}
	return v
}
func present(result gateway.Result, o outputOptions) gateway.Result {
	if result.IsError {
		return result
	}
	data, exists := result.Value["data"]
	if !exists {
		return result
	}
	values := map[string]any{}
	for k, v := range result.Value {
		values[k] = v
	}
	result.Value = values
	values["summary"] = structuralSummary(data)
	if o.Mode == "summary" {
		delete(values, "data")
		values["presentation"] = map[string]any{"mode": "summary", "data_omitted": true, "gateway_request_unchanged": true}
		return result
	}
	if o.Mode == "compact" {
		path := ""
		if o.ItemsPath != nil {
			path = *o.ItemsPath
		}
		target, ok := lookupOutput(data, path)
		var projected any
		var truncated bool
		if ok {
			projected, truncated, ok = projectTarget(target, o)
		}
		if !ok {
			failure := gateway.Failure("openany_mcp_output_unavailable", "API execution completed, but the requested output path/type is unavailable. Do not rerun automatically.", "result", "completed", values["http_status"], values["request_id"])
			failure.Value["summary"] = values["summary"]
			failure.Value["data"] = data
			failure.Value["presentation"] = map[string]any{"mode": "raw", "requested_mode": "compact", "applied": false}
			return failure
		}
		values["data"] = replaceOutput(data, path, projected)
		values["presentation"] = map[string]any{"mode": "compact", "items_path": path, "truncated": truncated, "gateway_request_unchanged": true}
	}
	return result
}
func withRecovery(result gateway.Result) gateway.Result {
	e, ok := result.Value["error"].(map[string]any)
	if !ok {
		return result
	}
	switch e["execution_state"] {
	case "unknown":
		e["next_action"] = "Do not retry automatically: execution or charging may have occurred. Save request_id and check the gateway."
	case "completed":
		e["next_action"] = "Execution completed. Inspect the available result and request_id; do not rerun to fix presentation."
	case "reported_failure":
		e["next_action"] = "Inspect the gateway error code and request_id. Check credentials, permission, quota and API parameters before deciding on another call."
	default:
		switch e["code"] {
		case "openany_mcp_invalid_search":
			e["next_action"] = "Correct search_apis query, filters or limit according to the error. For an invalid or outdated cursor, omit only cursor, keep the current query, filters and limit, and restart search_apis from the first page. The gateway was not called."
		case "openany_mcp_api_changed":
			e["next_action"] = "Call get_api again, review the current contract and price, then decide whether to execute."
		case "openany_mcp_api_unavailable":
			e["next_action"] = "Call search_apis to select a currently published model or API."
		case "openany_mcp_busy":
			e["next_action"] = "This call did not start. Wait before submitting another call."
		case "openany_mcp_invalid_output":
			e["next_action"] = "Correct output options. The gateway was not called."
		default:
			e["next_action"] = "Review get_api.input_schema and limitations; correct the request before trying again. The gateway was not called."
			if e["details_scope"] == "arguments" {
				e["next_action"] = "Correct the current MCP tool's arguments using its inputSchema and error.details. The gateway was not called."
			}
		}
	}
	return result
}
