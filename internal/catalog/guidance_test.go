package catalog

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/openany/openany-mcp/internal/model"
)

func guidanceFixture(t *testing.T, samples []PublicSample) *Discovery {
	t.Helper()
	name, root := "demo_company", ""
	r := model.Record{APIID: "id", Revision: 2, Name: "Company", Description: "Lookup company", ProviderID: "demo", ToolName: &name, Status: "active", Method: "POST", RequestContentType: "application/json", PublicPath: "/v1/agent-apis/demo/company/{id}", PathTemplate: "/company/{id}", ConfigPresent: true, UnknownParameters: "reject", Inputs: []model.Input{
		{Name: "id", Location: "path", Type: "integer", Required: true},
		{Name: "count", Location: "query", Type: "integer"},
		{Name: "X-Language", Location: "header", Type: "string"},
		{Name: "body", Location: "body", Pointer: &root, Required: true, Schema: json.RawMessage(`{"type":"object","$defs":{"record":{"type":"object","required":["number","nullable"],"properties":{"number":{"type":"integer"},"nullable":{"type":["string","null"]}}}},"required":["record"],"properties":{"record":{"$ref":"#/$defs/record"},"optional":{"type":"string"}}}`)},
	}}
	s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	return NewDiscovery(s, map[string]map[string]any{"id": {"mode": "fixed", "currency": "USD", "price_micro_usd": "1234", "unit": "request", "charge_condition": map[string]any{"source": "http_status", "op": "eq", "value": json.Number("200")}}}, true, map[string]APIMetadata{"id": {Revision: 2, Samples: samples}})
}
func goodSample() PublicSample {
	return PublicSample{Name: "public example", Request: map[string]any{"path": map[string]any{"id": "42"}, "query": map[string]any{"count": "10"}, "headers": map[string]any{"x-language": "zh-CN", "Authorization": "Bearer secret-credential"}, "body": map[string]any{"record": map[string]any{"number": json.Number("9007199254740993"), "nullable": nil}, "optional": "remove me"}}, HTTPStatus: 200, ResponseShape: map[string]any{"type": "object", "fields": []any{"results"}}}
}
func TestDetailOffersOnlyValidatedMinimalMCPExamples(t *testing.T) {
	d := guidanceFixture(t, []PublicSample{goodSample()})
	detail, _ := d.Detail("demo_company")
	examples, ok := detail["examples"].(map[string]any)
	if !ok {
		t.Fatal("examples missing")
	}
	if examples["status"] != "available" || examples["validation"] != "mcp_schema_and_http_encoding" {
		t.Fatal(examples)
	}
	items := examples["items"].([]any)
	input := items[0].(map[string]any)["input"].(map[string]any)
	if _, ok := input["count"]; ok {
		t.Fatal("optional query survived minimal example")
	}
	if _, err := d.snapshot.Tools["demo_company"].Input.Validate(mustJSON(t, input)); err != nil {
		t.Fatal(err)
	}
	raw := string(mustJSON(t, detail))
	if !strings.Contains(raw, "9007199254740993") || !strings.Contains(raw, `"nullable":null`) || strings.Contains(raw, "secret-credential") || strings.Contains(raw, "remove me") {
		t.Fatal("example altered exact values, kept optional fields or leaked authentication")
	}
	price := detail["price_summary"].(map[string]any)
	if price["amount_usd"] != "0.001234" || price["is_quote"] != false {
		t.Fatal(price)
	}
	if detail["required_inputs"] == nil || detail["response_hint"] == nil {
		t.Fatal("missing input/output guidance")
	}
}
func TestDetailRejectsSensitiveInvalidAndStaleSamples(t *testing.T) {
	sensitive := goodSample()
	sensitive.Request["body"] = map[string]any{"record": map[string]any{"number": json.Number("1"), "nullable": nil}, "password": "secret-password"}
	invalid := goodSample()
	invalid.Request["path"] = map[string]any{"id": "wrong-private-value"}
	for _, sample := range []PublicSample{sensitive, invalid} {
		detail, _ := guidanceFixture(t, []PublicSample{sample}).Detail("demo_company")
		examples := detail["examples"].(map[string]any)
		if examples["status"] != "no_safe_valid_sample" {
			t.Fatal(examples)
		}
		if raw := string(mustJSON(t, detail)); strings.Contains(raw, "secret-password") || strings.Contains(raw, "wrong-private-value") {
			t.Fatal("unsafe sample leaked")
		}
	}
	d := guidanceFixture(t, []PublicSample{goodSample()})
	b := d.snapshot.Tools["demo_company"]
	b.Revision++
	d = NewDiscovery(d.snapshot, nil, false, map[string]APIMetadata{"id": {Revision: 2, Samples: []PublicSample{goodSample()}}})
	detail, _ := d.Detail("demo_company")
	if detail["examples"].(map[string]any)["status"] == "available" {
		t.Fatal("stale sample published")
	}
}
func TestGuidanceDoesNotFabricateMissingSamplesOrUsageQuote(t *testing.T) {
	detail, _ := guidanceFixture(t, nil).Detail("demo_company")
	if detail["examples"].(map[string]any)["status"] != "missing" {
		t.Fatal(detail)
	}
	p := priceSummary(map[string]any{"status": "available", "mode": "usage", "price_micro_usd": "1", "unit_quantity": "1000", "unit": "token", "quantity_scale": json.Number("0")})
	if p["amount_usd"] != "0.000001" || p["is_quote"] != false || p["per_quantity"] != "1000" {
		t.Fatal(p)
	}
	huge := priceSummary(map[string]any{"status": "available", "mode": "fixed", "price_micro_usd": "9007199254740993123456", "unit": "request"})
	if huge["amount_usd"] != "9007199254740993.123456" {
		t.Fatal("price rounded", huge)
	}
}
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestSampleProjectionRespectsBodyPointersAndArrayEncoding(t *testing.T) {
	d := guidanceFixture(t, nil)
	b := d.snapshot.Tools["demo_company"]
	pointer := "/payload/a~1b"
	b.Request.Inputs = []model.Parameter{{Name: "body", Location: "body", Pointer: &pointer}, {Name: "id", Location: "path"}}
	args, ok := sampleArguments(b, map[string]any{"path": map[string]any{"id": "7"}, "body": map[string]any{"payload": map[string]any{"a/b": map[string]any{"record": map[string]any{"number": json.Number("1"), "nullable": nil}}}}})
	if !ok || args["id"] != json.Number("7") {
		t.Fatal("pointer projection failed", args)
	}
	props := b.Input.Document["properties"].(map[string]any)
	props["ids"] = map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}
	b.Request.QueryArrays = map[string]model.ArrayPolicy{"ids": {Encoding: "csv", Empty: "reject"}}
	value, ok := sampleValue(b, model.Parameter{Name: "ids", Location: "query"}, "1,9007199254740993")
	if !ok || value.([]any)[1] != json.Number("9007199254740993") {
		t.Fatal(value)
	}
	delete(b.Request.QueryArrays, "ids")
	if _, ok := sampleValue(b, model.Parameter{Name: "ids", Location: "query"}, "1,2"); ok {
		t.Fatal("guessed absent encoding")
	}
}

func TestRequiredHintsDoNotTreatNullableChildrenAsUnconditional(t *testing.T) {
	root := map[string]any{"type": "object", "required": []any{"body"}, "properties": map[string]any{"body": map[string]any{"type": []any{"object", "null"}, "required": []any{"company"}, "properties": map[string]any{"company": map[string]any{"type": "string"}}}}}
	out := []any{}
	requiredHints(root, root, "", "request", 0, &out)
	for _, v := range out {
		hint := v.(map[string]any)
		if hint["path"] == "/body/company" && hint["scope"] != "if_parent_is_object" {
			t.Fatal("nullable parent makes child conditional", hint)
		}
	}
}

func TestDetailDoesNotMutateCachedPublicSample(t *testing.T) {
	sample := goodSample()
	before := mustJSON(t, sample.Request)
	d := guidanceFixture(t, []PublicSample{sample})
	a, _ := d.Detail("demo_company")
	b, _ := d.Detail("demo_company")
	if string(before) != string(mustJSON(t, sample.Request)) || string(mustJSON(t, a)) != string(mustJSON(t, b)) {
		t.Fatal("get_api changed cached metadata")
	}
}
