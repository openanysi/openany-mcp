package schema

import (
	"errors"
	"strings"
	"unicode"

	js "github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

type FieldError struct {
	Path    string `json:"path"`
	Keyword string `json:"keyword"`
	Message string `json:"message"`
}

// ValidationDetails deliberately does not call Error/LocalizedString: enum, pattern
// and other library messages can contain the submitted value or private schema data.
func ValidationDetails(err error) []FieldError {
	out := []FieldError{}
	var validation *js.ValidationError
	if !errors.As(err, &validation) {
		return []FieldError{{"", "json", "Supply one valid JSON object without duplicate keys."}}
	}
	add := func(parts []string, keyword, message string) {
		if len(out) >= 10 {
			return
		}
		path := ""
		for _, part := range parts {
			if len(part) > 80 || strings.IndexFunc(part, unicode.IsControl) >= 0 {
				part = "[field]"
			}
			path += "/" + escapePointer(part)
			if len(path) > 512 {
				path = ""
				break
			}
		}
		out = append(out, FieldError{path, keyword, message})
	}
	var visit func(*js.ValidationError)
	visit = func(e *js.ValidationError) {
		if len(out) >= 10 {
			return
		}
		if len(e.Causes) > 0 {
			for _, child := range e.Causes {
				visit(child)
			}
			return
		}
		parts := e.InstanceLocation
		if k, ok := e.ErrorKind.(*kind.Required); ok {
			for _, name := range k.Missing {
				p := append(append([]string{}, parts...), name)
				add(p, "required", "Supply this required field.")
			}
			return
		}
		keyword := "schema"
		message := "Value violates this schema constraint; inspect input_schema."
		if p := e.ErrorKind.KeywordPath(); len(p) > 0 {
			switch p[0] {
			case "type":
				keyword = "type"
				if k, ok := e.ErrorKind.(*kind.Type); ok {
					message = "Use type: " + strings.Join(k.Want, " or ") + "."
				}
			case "enum", "const":
				keyword = p[0]
				message = "Use an allowed value from input_schema."
			case "additionalProperties", "unevaluatedProperties":
				keyword = p[0]
				message = "Remove properties not allowed by input_schema."
			case "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength", "pattern", "format", "minItems", "maxItems", "uniqueItems", "minProperties", "maxProperties", "dependentRequired", "oneOf", "anyOf", "not", "falseSchema":
				keyword = p[0]
			}
		}
		add(parts, keyword, message)
	}
	visit(validation)
	return out
}
