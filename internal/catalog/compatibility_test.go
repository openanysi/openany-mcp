package catalog

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/config"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/transport"
	"net/url"
	"os"
	"strings"
	"testing"
)

func compatibilityRecords(t *testing.T) []model.Record {
	t.Helper()
	raw, err := os.ReadFile("testdata/compatibility/hunter-alchemy.json")
	if err != nil {
		t.Fatal(err)
	}
	var records []model.Record
	if err = json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	return records
}
func compatibilitySnapshot(t *testing.T, record model.Record) *Snapshot {
	t.Helper()
	o, err := config.ReadOverrides("../../config/catalog-overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Build([]model.Record{record}, o, false)
	if err != nil || len(s.Tools) != 1 {
		t.Fatalf("published contract quarantined: %v, issues=%+v", err, s.Issues)
	}
	return s
}
func TestHunterFormCatalogCompilesPublishedInputs(t *testing.T) {
	r := compatibilityRecords(t)[0]
	s := compatibilitySnapshot(t, r)
	b := s.Tools[*r.ToolName]
	args, err := b.Input.Validate([]byte(`{"company":"中国 A&B+Corp","limit":2,"perfect_match":false}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := transport.Encode(b.Request, args)
	if err != nil {
		t.Fatal(err)
	}
	form, err := url.ParseQuery(string(encoded.Body))
	if err != nil {
		t.Fatal(err)
	}
	if encoded.Headers.Get("Content-Type") != "application/x-www-form-urlencoded" || form.Get("company") != "中国 A&B+Corp" || form.Get("limit") != "2" || form.Get("perfect_match") != "false" || len(form) != 3 {
		t.Fatalf("wrong form: %q %+v", encoded.Body, form)
	}
	for _, bad := range []string{`{}`, `{"company":null}`, `{"company":{}}`, `{"company":"valid","limit":11}`, `{"company":"valid","perfect_match":"false"}`} {
		if _, err := b.Input.Validate([]byte(bad)); err == nil {
			t.Fatalf("invalid input accepted: %s", bad)
		}
	}
	if r.RequestContentType != "application/x-www-form-urlencoded" {
		t.Fatal("source contract mutated")
	}
}
func TestHunterFormRejectsUnreviewedComplexShapes(t *testing.T) {
	for _, change := range []func(*model.Record){func(r *model.Record) { r.Method = "GET" },
		func(r *model.Record) { r.Method = "GET"; r.Inputs = nil },
		func(r *model.Record) { r.Inputs[0].Schema = json.RawMessage(`{"type":"object"}`) }, func(r *model.Record) { r.Inputs[0].Type = "object" }, func(r *model.Record) { r.Inputs[0].Type = "array" }, func(r *model.Record) { r.Inputs[0].Nullable = true }, func(r *model.Record) { p := "/company/name"; r.Inputs[0].Pointer = &p }, func(r *model.Record) { p := ""; r.Inputs[0].Pointer = &p }} {
		r := compatibilityRecords(t)[0]
		change(&r)
		s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
		if err != nil || len(s.Tools) != 0 || s.Quarantined != 1 {
			t.Fatalf("ambiguous form accepted: %+v %v", s, err)
		}
	}
}
func TestAlchemyReviewedCSVPolicy(t *testing.T) {
	r := compatibilityRecords(t)[1]
	b := compatibilitySnapshot(t, r).Tools[*r.ToolName]
	args, err := b.Input.Validate([]byte(`{"symbols":["ETH","BTC"]}`))
	if err != nil {
		t.Fatal(err)
	}
	e, err := transport.Encode(b.Request, args)
	if err != nil || e.Query != "symbols=ETH%2CBTC" || len(e.Body) != 0 {
		t.Fatalf("not reviewed CSV: %q %v", e.Query, err)
	}
	for _, bad := range []string{`{}`, `{"symbols":[]}`, `{"symbols":"ETH,BTC"}`, `{"symbols":[1]}`} {
		if _, err := b.Input.Validate([]byte(bad)); err == nil {
			t.Fatalf("invalid array accepted: %s", bad)
		}
	}
	if _, err := transport.Encode(b.Request, map[string]any{"symbols": []any{"ETH,BTC"}}); err == nil {
		t.Fatal("ambiguous delimiter accepted")
	}
	values := make([]any, 25)
	for i := range values {
		values[i] = "ETH"
	}
	encoded, err := transport.Encode(b.Request, map[string]any{"symbols": values})
	if err != nil {
		t.Fatal(err)
	}
	query, _ := url.ParseQuery(encoded.Query)
	if len(strings.Split(query.Get("symbols"), ",")) != 25 {
		t.Fatal("valid 25-symbol boundary changed")
	}
}

func TestAlchemyCSVPolicyIsScopedByProviderMethodAndPath(t *testing.T) {
	for _, field := range []string{"provider", "method", "path"} {
		r := compatibilityRecords(t)[1]
		switch field {
		case "provider":
			r.ProviderID = "other"
			r.PublicPath = "/v1/agent-apis/other" + r.PathTemplate
		case "method":
			r.Method = "POST"
		case "path":
			r.PathTemplate = "/other"
			r.PublicPath = "/v1/agent-apis/alchemy/other"
		}
		o, err := config.ReadOverrides("../../config/catalog-overrides.json")
		if err != nil {
			t.Fatal(err)
		}
		s, err := Build([]model.Record{r}, o, false)
		if err != nil || len(s.Tools) != 0 || s.Quarantined != 1 {
			t.Fatalf("CSV policy leaked across %s", field)
		}
	}
}
