package catalog

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/schema"
	"github.com/openany/openany-mcp/internal/transport"
)

// Examples are public HTTP samples projected onto declared MCP inputs. Never synthesize
// credentials or guess an upstream-specific request when the source example is missing.
func guidance(b *Binding, e discoveryEntry) map[string]any {
	examples := map[string]any{"status": "missing", "items": []any{}, "validation": "mcp_schema_and_http_encoding", "supplier_verified": false}
	if e.samplesStale {
		examples["status"] = "stale"
	}
	items, templates := []any{}, []any{}
	for i, s := range e.metadata.Samples {
		if i >= 3 {
			break
		}
		args, ok := sampleArguments(b, s.Request)
		if !ok {
			continue
		}
		item := map[string]any{"name": publicText(s.Name, 80), "input": args, "kind": "example"}
		if paths := placeholderPaths(args); len(paths) > 0 {
			item["kind"], item["placeholder_paths"] = "template", paths
			templates = append(templates, item)
		} else {
			items = append(items, item)
		}
	}
	if len(e.metadata.Samples) > 0 {
		examples["status"] = "no_safe_valid_sample"
	}
	if len(items) == 0 && !e.samplesStale && validExample(b, map[string]any{}) {
		items = append(items, map[string]any{"name": "Empty request (schema and HTTP encoding validated; not executed)", "input": map[string]any{}, "kind": "empty_request"})
	}
	if len(items) > 0 {
		examples["status"] = "available"
	} else if len(templates) > 0 {
		examples["status"] = "template_only"
	}
	examples["items"] = append(items, templates...)
	hints := []any{}
	requiredHints(b.Input.Document, b.Input.Document, "", "request", 0, &hints)
	response := map[string]any{"status": "unavailable", "source": "public_sample_structure", "authoritative": false}
	for _, s := range e.metadata.Samples {
		if len(s.ResponseShape) == 0 {
			continue
		}
		// A response sample describes an example, never a guaranteed response contract.
		shape := object(s.ResponseShape, "type", "fields", "items")
		response = map[string]any{"status": "available", "source": "public_sample_structure", "authoritative": false, "sample_http_status": s.HTTPStatus, "shape": shape}
		break
	}
	response["supported_media_types"] = []string{"application/json", "application/*+json", "text/plain", "text/markdown"}
	return map[string]any{"examples": examples, "required_inputs": hints, "input_guidance": "Read input_schema for conditional and cross-field constraints. Examples preserve public business parameters and pass the current MCP schema and HTTP encoder; upstream acceptance is not verified. Templates require replacement of placeholder_paths. An empty_request accepts input {} locally. supported_media_types describes service capabilities, not this API's guaranteed response format.", "price_summary": priceSummary(e.pricing), "response_hint": response}
}
func pointerPart(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
func atPointer(v any, p string) (any, bool) {
	if p == "" {
		return v, true
	}
	if !strings.HasPrefix(p, "/") {
		return nil, false
	}
	for _, part := range strings.Split(p[1:], "/") {
		if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(part, "~0", ""), "~1", ""), "~") {
			return nil, false
		}
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		switch x := v.(type) {
		case map[string]any:
			var ok bool
			v, ok = x[part]
			if !ok {
				return nil, false
			}
		case []any:
			i, e := strconv.Atoi(part)
			if e != nil || i < 0 || i >= len(x) || (len(part) > 1 && part[0] == '0') {
				return nil, false
			}
			v = x[i]
		default:
			return nil, false
		}
	}
	return v, true
}
func resolved(root map[string]any, doc map[string]any) map[string]any {
	// Resolve local references for hints/coercion only; the compiled validator remains authoritative.
	for i := 0; i < 8; i++ {
		ref, _ := doc["$ref"].(string)
		if !strings.HasPrefix(ref, "#/") {
			break
		}
		v, ok := atPointer(root, ref[1:])
		if !ok {
			break
		}
		next, ok := v.(map[string]any)
		if !ok {
			break
		}
		doc = next
	}
	return doc
}
func hasType(doc map[string]any, t string) bool {
	if doc["type"] == t {
		return true
	}
	if a, ok := doc["type"].([]any); ok {
		for _, v := range a {
			if v == t {
				return true
			}
		}
	}
	return false
}
func sampleValue(b *Binding, p model.Parameter, value any) (any, bool) {
	props, _ := b.Input.Document["properties"].(map[string]any)
	doc, _ := props[p.Name].(map[string]any)
	doc = resolved(b.Input.Document, doc)
	if p.Location == "body" {
		return value, true
	}
	if hasType(doc, "array") {
		policy, ok := b.Request.QueryArrays[p.Name]
		if p.Location != "query" || !ok {
			return nil, false
		}
		var a []any
		switch x := value.(type) {
		case []any:
			a = x
		case string:
			switch policy.Encoding {
			case "csv":
				for _, s := range strings.Split(x, ",") {
					a = append(a, s)
				}
			case "json":
				v, err := schema.Decode([]byte(x))
				if err != nil {
					return nil, false
				}
				a, ok = v.([]any)
				if !ok {
					return nil, false
				}
			default:
				return nil, false
			}
		default:
			return nil, false
		}
		item, _ := doc["items"].(map[string]any)
		item = resolved(b.Input.Document, item)
		out := []any{}
		for _, v := range a {
			out = append(out, coerceScalar(item, v))
		}
		return out, true
	}
	return coerceScalar(doc, value), true
}
func coerceScalar(doc map[string]any, v any) any {
	s, ok := v.(string)
	if !ok || hasType(doc, "string") {
		return v
	}
	if hasType(doc, "integer") || hasType(doc, "number") || hasType(doc, "boolean") {
		parsed, err := schema.Decode([]byte(s))
		if err == nil {
			switch parsed.(type) {
			case json.Number, bool:
				return parsed
			}
		}
	}
	return v
}
func sensitive(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			key := strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(k))
			switch key {
			case "authorization", "apikey", "accesstoken", "refreshtoken", "token", "password", "secret", "clientsecret", "privatekey", "cookie":
				return true
			}
			if sensitive(v) {
				return true
			}
		}
	case []any:
		for _, v := range x {
			if sensitive(v) {
				return true
			}
		}
	case string:
		if credentialText.MatchString(x) {
			return true
		}
	}
	return false
}
func sampleArguments(b *Binding, request map[string]any) (map[string]any, bool) {
	raw, err := json.Marshal(request)
	if err != nil || len(raw) > 16384 {
		return nil, false
	}
	args := map[string]any{}
	for _, p := range b.Request.Inputs {
		var value any
		var exists bool
		if p.Location == "body" {
			body, ok := request["body"]
			if !ok {
				continue
			}
			ptr := "/" + pointerPart(p.Name)
			if p.Pointer != nil {
				ptr = *p.Pointer
			}
			value, exists = atPointer(body, ptr)
		} else {
			group := p.Location
			if group == "header" {
				group = "headers"
			}
			values, _ := request[group].(map[string]any)
			if p.Location == "header" {
				if transport.ReservedHeader(p.Name) {
					continue
				}
				for k, v := range values {
					if strings.EqualFold(k, p.Name) {
						value, exists = v, true
						break
					}
				}
			} else {
				value, exists = values[p.Name]
			}
		}
		if !exists {
			continue
		}
		value, exists = sampleValue(b, p, value)
		if !exists {
			return nil, false
		}
		args[p.Name] = value
	}
	// Ignore undeclared transport auth headers, reject credential-shaped declared values.
	if sensitive(args) || !validExample(b, args) {
		return nil, false
	}
	// Projected bodies may still reference snapshot maps; detail results must own their values.
	raw, err = json.Marshal(args)
	if err != nil {
		return nil, false
	}
	cloned, err := schema.Decode(raw)
	if err != nil {
		return nil, false
	}
	return cloned.(map[string]any), true
}
func validExample(b *Binding, args map[string]any) bool {
	raw, err := json.Marshal(args)
	if err != nil {
		return false
	}
	if _, err = b.Input.Validate(raw); err != nil {
		return false
	}
	_, err = transport.Encode(b.Request, args)
	return err == nil
}

var templateValue = regexp.MustCompile(`^(?:<[A-Za-z_][A-Za-z0-9_. -]{0,79}>|\{\{[A-Za-z_][A-Za-z0-9_. -]{0,79}\}\}|\$\{[A-Za-z_][A-Za-z0-9_. -]{0,79}\}|YOUR_[A-Z0-9_]+|REPLACE_[A-Z0-9_]+)$`)

func placeholderPaths(value any) []string {
	paths := []string{}
	var walk func(any, string)
	walk = func(value any, path string) {
		if len(paths) >= 40 {
			return
		}
		switch v := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				walk(v[key], path+"/"+pointerPart(key))
			}
		case []any:
			for i, item := range v {
				walk(item, path+"/"+strconv.Itoa(i))
			}
		case string:
			if templateValue.MatchString(strings.TrimSpace(v)) {
				paths = append(paths, path)
			}
		}
	}
	walk(value, "")
	return paths
}
func requiredHints(root, doc map[string]any, path, scope string, depth int, out *[]any) {
	if depth >= 8 || len(*out) >= 40 {
		return
	}
	doc = resolved(root, doc)
	props, _ := doc["properties"].(map[string]any)
	req, _ := doc["required"].([]any)
	for _, value := range req {
		name, ok := value.(string)
		if !ok || len(*out) >= 40 {
			continue
		}
		field, _ := props[name].(map[string]any)
		field = resolved(root, field)
		p := path + "/" + pointerPart(name)
		*out = append(*out, map[string]any{"path": p, "type": field["type"], "description": publicText(stringValue(field["description"]), 240), "scope": scope})
		requiredHints(root, field, p, "if_parent_is_object", depth+1, out)
		if items, ok := field["items"].(map[string]any); ok {
			requiredHints(root, items, p+"/*", "each_present_item", depth+1, out)
		}
	}
}
func stringValue(v any) string { s, _ := v.(string); return s }
func usd(micro string) string {
	if micro == "" || strings.Trim(micro, "0123456789") != "" {
		return ""
	}
	micro = strings.TrimLeft(micro, "0")
	if micro == "" {
		return "0"
	}
	for len(micro) <= 6 {
		micro = "0" + micro
	}
	whole, frac := micro[:len(micro)-6], strings.TrimRight(micro[len(micro)-6:], "0")
	if frac == "" {
		return whole
	}
	return whole + "." + frac
}
func priceSummary(p map[string]any) map[string]any {
	out := map[string]any{"status": p["status"], "is_quote": false, "settlement": "gateway"}
	if p["status"] != "available" {
		out["description"] = "Public pricing is unavailable. Do not infer that this API is free."
		return out
	}
	mode := stringValue(p["mode"])
	out["mode"] = mode
	if mode == "rules" {
		out["description"] = "Rule-based pricing: inspect pricing.policy.sale variants, addons and quantities. The gateway computes the charge; a base price is not the final total."
		return out
	}
	amount := usd(stringValue(p["price_micro_usd"]))
	out["amount_usd"] = amount
	out["unit"] = p["unit"]
	quantity := "1"
	if mode == "usage" {
		quantity = stringValue(p["unit_quantity"])
		if quantity == "" {
			quantity = "unknown"
		}
		out["per_quantity"] = quantity
		out["quantity_scale"] = p["quantity_scale"]
		out["rounding"] = p["rounding"]
	}
	out["description"] = "USD " + amount + " per " + quantity + " " + stringValue(p["unit"]) + "; actual charge follows pricing.charge_condition and gateway settlement."
	if mode == "usage" {
		out["description"] = out["description"].(string) + " Usage, scaling and rounding affect the final charge."
	}
	if p["charge_condition"] != nil {
		out["charge_condition"] = p["charge_condition"]
	}
	return out
}
