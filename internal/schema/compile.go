package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/openany/openany-mcp/internal/model"
	js "github.com/santhosh-tekuri/jsonschema/v6"
	"io"
	"strconv"
	"strings"
)

type Compiled struct {
	Document  map[string]any
	Validator *js.Schema
}

// Decode rejects duplicate keys rather than accepting encoding/json's last value.
func Decode(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	v, err := decodeValue(d, 0)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	return v, nil
}
func decodeValue(d *json.Decoder, depth int) (any, error) {
	if depth >= 128 {
		return nil, fmt.Errorf("JSON nesting exceeds 128")
	}
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		m := map[string]any{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return nil, e
			}
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("invalid object key")
			}
			if _, exists := m[key]; exists {
				return nil, fmt.Errorf("duplicate JSON key")
			}
			v, e := decodeValue(d, depth+1)
			if e != nil {
				return nil, e
			}
			m[key] = v
		}
		_, err = d.Token()
		return m, err
	case '[':
		a := []any{}
		for d.More() {
			v, e := decodeValue(d, depth+1)
			if e != nil {
				return nil, e
			}
			a = append(a, v)
		}
		_, err = d.Token()
		return a, err
	}
	return nil, fmt.Errorf("invalid delimiter")
}
func (c *Compiled) Validate(raw []byte) (map[string]any, error) {
	v, err := Decode(raw)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("arguments must be object")
	}
	if err = c.Validator.Validate(m); err != nil {
		return nil, err
	}
	return m, nil
}
func Compile(r model.Record) (*Compiled, error) {
	props := map[string]any{}
	required := []any{}
	for _, in := range r.Inputs {
		if in.Name == "" {
			return nil, fmt.Errorf("empty input name")
		}
		if _, ok := props[in.Name]; ok {
			return nil, fmt.Errorf("duplicate input name %s", in.Name)
		}
		var doc map[string]any
		if len(in.Schema) > 0 && string(in.Schema) != "null" {
			v, e := Decode(in.Schema)
			if e != nil {
				return nil, e
			}
			var ok bool
			doc, ok = v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("schema must be object")
			}
		} else {
			doc = map[string]any{"type": in.Type}
			if in.Type == "array" {
				item := in.ItemType
				if item == "" {
					item = "string"
				}
				// OpenAny's "any" permits every JSON value; JSON Schema has no "any" type.
				if item == "any" {
					doc["items"] = map[string]any{}
				} else {
					doc["items"] = map[string]any{"type": item}
				}
			}
		}
		ns := "#/properties/" + escapePointer(in.Name)
		if err := normalize(doc, ns, false); err != nil {
			return nil, err
		}
		if len(in.Enum) > 0 && string(in.Enum) != "null" {
			v, e := Decode(in.Enum)
			if e != nil {
				return nil, e
			}
			doc["enum"] = v
		}
		for key, raw := range map[string]json.RawMessage{"minimum": in.Minimum, "maximum": in.Maximum} {
			if len(raw) == 0 || string(raw) == "null" {
				continue
			}
			v, e := Decode(raw)
			if e != nil {
				return nil, e
			}
			if s, ok := v.(string); ok {
				if _, e = strconv.ParseFloat(s, 64); e != nil {
					return nil, fmt.Errorf("invalid %s", key)
				}
				v = json.Number(s)
			}
			doc[key] = v
		}
		if in.Description != "" {
			doc["description"] = in.Description
		}
		if in.Nullable {
			if err := makeNullable(doc); err != nil {
				return nil, err
			}
		}

		props[in.Name] = doc
		if in.Required {
			required = append(required, in.Name)
		}
	}
	root := map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": props, "additionalProperties": r.UnknownParameters == "ignore"}
	if len(required) > 0 {
		root["required"] = required
	}
	compiler := js.NewCompiler()
	compiler.DefaultDraft(js.Draft2020)
	compiler.UseLoader(nil)
	if err := compiler.AddResource("urn:openany:input", root); err != nil {
		return nil, err
	}
	validator, err := compiler.Compile("urn:openany:input")
	if err != nil {
		return nil, err
	}
	return &Compiled{root, validator}, nil
}
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
func normalize(s map[string]any, ns string, draft07 bool) error {
	if _, ok := s["$id"]; ok {
		return fmt.Errorf("embedded $id requires bundling")
	}
	if dialect, ok := s["$schema"].(string); ok {
		switch strings.TrimSuffix(dialect, "#") {
		case "http://json-schema.org/draft-07/schema", "https://json-schema.org/draft-07/schema":
			draft07 = true
		case "https://json-schema.org/draft/2020-12/schema", "http://json-schema.org/draft/2020-12/schema":
			draft07 = false
		default:
			return fmt.Errorf("unsupported source schema dialect")
		}
	}
	if draft07 {
		if _, ref := s["$ref"]; ref {
			for key := range s {
				switch key {
				case "$ref", "$schema", "$defs", "definitions", "title", "description", "$comment":
				default:
					delete(s, key)
				}
			}
		}
		// These are annotations in draft-07, not validation constraints.
		for _, key := range []string{"dependentRequired", "dependentSchemas", "prefixItems", "unevaluatedProperties", "unevaluatedItems", "minContains", "maxContains", "$dynamicRef", "$dynamicAnchor"} {
			delete(s, key)
		}
	}
	if _, ok := s["$id"]; ok {
		return fmt.Errorf("embedded $id requires bundling")
	}
	delete(s, "$schema")
	if ref, ok := s["$ref"].(string); ok {
		if !strings.HasPrefix(ref, "#") {
			return fmt.Errorf("external ref forbidden")
		}
		if strings.HasPrefix(ref, "#/definitions/") {
			ref = "#/$defs/" + strings.TrimPrefix(ref, "#/definitions/")
		}
		s["$ref"] = ns + strings.TrimPrefix(ref, "#")
	}
	if defs, ok := s["definitions"]; ok {
		if _, exists := s["$defs"]; exists {
			return fmt.Errorf("ambiguous definitions")
		}
		s["$defs"] = defs
		delete(s, "definitions")
	}
	if deps, ok := s["dependencies"].(map[string]any); ok && draft07 {
		dr := map[string]any{}
		ds := map[string]any{}
		for k, v := range deps {
			if _, ok := v.([]any); ok {
				dr[k] = v
			} else {
				ds[k] = v
			}
		}
		s["dependentRequired"] = dr
		s["dependentSchemas"] = ds
		delete(s, "dependencies")
	}
	if !draft07 {
		delete(s, "dependencies")
	}
	if tuple, ok := s["items"].([]any); ok {
		if !draft07 {
			return fmt.Errorf("tuple items requires a draft-07 source schema")
		}
		s["prefixItems"] = tuple
		if extra, ok := s["additionalItems"]; ok {
			s["items"] = extra
		} else {
			delete(s, "items")
		}
		delete(s, "additionalItems")
	}
	for _, key := range []string{"properties", "patternProperties", "$defs", "dependentSchemas"} {
		if m, ok := s[key].(map[string]any); ok {
			for _, child := range m {
				if c, ok := child.(map[string]any); ok {
					if err := normalize(c, ns, draft07); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, key := range []string{"items", "contains", "additionalProperties", "propertyNames", "not", "if", "then", "else", "unevaluatedProperties", "unevaluatedItems"} {
		if child, ok := s[key].(map[string]any); ok {
			if err := normalize(child, ns, draft07); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf", "prefixItems"} {
		if a, ok := s[key].([]any); ok {
			for _, v := range a {
				if c, ok := v.(map[string]any); ok {
					if err := normalize(c, ns, draft07); err != nil {
						return err
					}
				}
			}
		}
	}
	if nullable, ok := s["nullable"].(bool); ok {
		delete(s, "nullable")
		if nullable {
			return makeNullable(s)
		}
	}
	return nil
}
func makeNullable(s map[string]any) error {
	if enum, ok := s["enum"].([]any); ok {
		hasNull := false
		for _, v := range enum {
			if v == nil {
				hasNull = true
			}
		}
		if !hasNull {
			return fmt.Errorf("nullable conflicts with enum")
		}
	}
	switch value := s["type"].(type) {
	case string:
		if value != "null" {
			s["type"] = []any{value, "null"}
		}
		return nil
	case []any:
		for _, v := range value {
			if v == "null" {
				return nil
			}
		}
		s["type"] = append(value, "null")
		return nil
	}
	// Definitions retain their original location when only the constraint is wrapped.
	base := map[string]any{}
	for k, v := range s {
		if k != "$defs" {
			base[k] = v
			delete(s, k)
		}
	}
	s["anyOf"] = []any{base, map[string]any{"type": "null"}}
	return nil
}
