package schema

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/model"
	"testing"
)

func TestCompileRelocatesReferencesAndRejectsWrongTypes(t *testing.T) {
	r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Location: "body", Type: "object", Required: true, Schema: json.RawMessage(`{"type":"object","$defs":{"N":{"type":"integer"}},"properties":{"n":{"$ref":"#/$defs/N"}},"required":["n"]}`)}}}
	c, err := Compile(r)
	if err != nil {
		t.Fatal(err)
	}
	args, err := c.Validate([]byte(`{"body":{"n":9007199254740993}}`))
	if err != nil {
		t.Fatal(err)
	}
	if args["body"].(map[string]any)["n"].(json.Number).String() != "9007199254740993" {
		t.Fatal("number lost precision")
	}
	for _, raw := range []string{`{"body":{"n":"1"}}`, `{"body":{"n":1},"extra":1}`, `{"body":{"n":1,"n":2}}`} {
		if _, err = c.Validate([]byte(raw)); err == nil {
			t.Fatalf("accepted invalid input %s", raw)
		}
	}
}
func TestCompileBoundsNullableAndExternalRefs(t *testing.T) {
	r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "limit", Location: "query", Type: "integer", Minimum: json.RawMessage(`"1"`), Maximum: json.RawMessage(`"10"`)}}}
	c, err := Compile(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Validate([]byte(`{"limit":11}`)); err == nil {
		t.Fatal("bound not enforced")
	}
	r.Inputs[0].Schema = json.RawMessage(`{"$ref":"https://untrusted.invalid/schema"}`)
	if _, err = Compile(r); err == nil {
		t.Fatal("external ref accepted")
	}
}
func TestNullableChildDoesNotRelocateRootDefinitions(t *testing.T) {
	r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Location: "body", Type: "object", Required: true, Schema: json.RawMessage(`{"type":"object","$defs":{"N":{"type":"integer"}},"properties":{"n":{"$ref":"#/$defs/N","nullable":true}}}`)}}}
	c, err := Compile(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"body":{"n":null}}`, `{"body":{"n":2}}`} {
		if _, err = c.Validate([]byte(raw)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestDraft07ReferenceSiblingsAreIgnored(t *testing.T) {
	r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Location: "body", Type: "integer", Schema: json.RawMessage(`{"$schema":"http://json-schema.org/draft-07/schema#","definitions":{"N":{"type":"integer"}},"$ref":"#/definitions/N","minimum":10}`)}}}
	c, err := Compile(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Validate([]byte(`{"body":1}`)); err != nil {
		t.Fatalf("draft07 ref siblings changed semantics: %v", err)
	}
}
func TestModernDependenciesDoNotOverrideDependentRequired(t *testing.T) {
	r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Location: "body", Type: "object", Schema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","dependencies":{"a":["b"]},"dependentRequired":{"a":["c"]}}`)}}}
	c, err := Compile(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Validate([]byte(`{"body":{"a":1,"c":2}}`)); err != nil {
		t.Fatal("legacy annotation overrode modern constraint", err)
	}
	if _, err = c.Validate([]byte(`{"body":{"a":1,"b":2}}`)); err == nil {
		t.Fatal("modern dependentRequired lost")
	}
}
func TestDraft07ReferenceCannotHideExternalResourceIdentity(t *testing.T) {
	r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Location: "body", Schema: json.RawMessage(`{"$schema":"http://json-schema.org/draft-07/schema#","$id":"https://untrusted.invalid/schema","$ref":"#","type":"string"}`)}}}
	if _, err := Compile(r); err == nil {
		t.Fatal("unsupported embedded resource identity accepted")
	}
}
func TestLocalReferenceDoesNotRenamePropertyCalledDefinitions(t *testing.T) {
	r := model.Record{UnknownParameters: "reject", Inputs: []model.Input{{Name: "body", Location: "body", Schema: json.RawMessage(`{"properties":{"definitions":{"N":{"type":"string"}},"$defs":{"N":{"type":"integer"}}},"$ref":"#/properties/definitions/N"}`)}}}
	c, err := Compile(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Validate([]byte(`{"body":"ok"}`)); err != nil {
		t.Fatalf("property name was treated as a schema keyword: %v", err)
	}
}
