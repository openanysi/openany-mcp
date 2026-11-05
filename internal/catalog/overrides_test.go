package catalog

import (
	"encoding/json"
	"testing"

	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/transport"
)

func routePolicy(t *testing.T) model.Overrides {
	t.Helper()
	var o model.Overrides
	if err := json.Unmarshal([]byte(`{"version":1,"routes":[{"provider_id":"demo","method":"GET","path_template":"/list","query_arrays":{"tags":{"encoding":"repeated","empty":"reject","source":"test contract"}}}]}`), &o); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestRoutePolicyWorksAcrossUUIDsAndRestrictsEmptyArrays(t *testing.T) {
	o := routePolicy(t)
	for _, id := range []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000099"} {
		r := fixture("demo_tags")
		r.APIID = id
		r.Inputs = []model.Input{{Name: "tags", Location: "query", Type: "array", Required: true}}
		s, err := Build([]model.Record{r}, o, false)
		if err != nil || len(s.Tools) != 1 {
			t.Fatalf("route policy did not resolve for %s: %v %+v", id, err, s)
		}
		b := s.Tools["demo_tags"]
		args, err := b.Input.Validate([]byte(`{"tags":["a","b"]}`))
		if err != nil {
			t.Fatal(err)
		}
		e, err := transport.Encode(b.Request, args)
		if err != nil || e.Query != "tags=a&tags=b" {
			t.Fatalf("wrong query encoding: %+v %v", e, err)
		}
		if _, err := b.Input.Validate([]byte(`{"tags":[]}`)); err == nil {
			t.Fatal("tool schema advertised unsupported empty arrays")
		}
	}
}

func TestUUIDPolicyOverridesRouteWithoutChangingOtherParameters(t *testing.T) {
	o := routePolicy(t)
	r := fixture("demo_tags")
	r.Inputs = []model.Input{{Name: "tags", Location: "query", Type: "array", Required: true}}
	o.APIs = map[string]model.Override{r.APIID: {Description: "Reviewed description", QueryArrays: map[string]model.ArrayEncoding{"tags": {Encoding: "csv", Empty: "empty", Source: "specific contract"}}}}
	s, err := Build([]model.Record{r}, o, false)
	if err != nil || len(s.Tools) != 1 {
		t.Fatal("specific policy not published", err)
	}
	b := s.Tools["demo_tags"]
	args, err := b.Input.Validate([]byte(`{"tags":["a","b"]}`))
	if err != nil {
		t.Fatal(err)
	}
	e, err := transport.Encode(b.Request, args)
	if err != nil || e.Query != "tags=a%2Cb" {
		t.Fatalf("UUID policy did not win: %q %v", e.Query, err)
	}
	if _, err := b.Input.Validate([]byte(`{"tags":[]}`)); err != nil {
		t.Fatal("route restriction survived explicit override", err)
	}
	delete(o.APIs, r.APIID)
	s, err = Build([]model.Record{r}, o, false)
	if err != nil || len(s.Tools) != 1 || s.Tools["demo_tags"].Request.QueryArrays["tags"].Encoding != "repeated" {
		t.Fatal("specific override mutated route defaults", err)
	}
}

func TestRoutePoliciesRejectDuplicateKeysAndDoNotMatchOtherRoutes(t *testing.T) {
	var o model.Overrides
	if err := json.Unmarshal([]byte(`{"version":1,"routes":[{"provider_id":"demo","method":"GET","path_template":"/list"},{"provider_id":"demo","method":"GET","path_template":"/list"}]}`), &o); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(nil, o, false); err == nil {
		t.Fatal("ambiguous duplicate route policy accepted")
	}
	for _, change := range []string{"provider", "method", "path"} {
		r := fixture("demo_tags")
		r.Inputs = []model.Input{{Name: "tags", Location: "query", Type: "array", Required: true}}
		switch change {
		case "provider":
			r.ProviderID, r.PublicPath = "other", "/v1/agent-apis/other/list"
		case "method":
			r.Method, r.RequestContentType = "POST", "application/json"
		case "path":
			r.PathTemplate, r.PublicPath = "/different", "/v1/agent-apis/demo/different"
		}
		s, err := Build([]model.Record{r}, routePolicy(t), false)
		if err != nil || len(s.Tools) != 0 || s.Quarantined != 1 {
			t.Fatalf("policy leaked to different %s: %v", change, err)
		}
	}
}

func TestRouteNonEmptyPolicyPreservesSchemaReferencesAndStrongerLimits(t *testing.T) {
	r := fixture("demo_tags")
	raw := json.RawMessage(`{"type":"array","$defs":{"Tag":{"type":"string"}},"items":{"$ref":"#/$defs/Tag"},"minItems":2,"maxItems":3}`)
	r.Inputs = []model.Input{{Name: "tags", Location: "query", Type: "array", Nullable: true, Schema: raw}}
	s, err := Build([]model.Record{r}, routePolicy(t), false)
	if err != nil || len(s.Tools) != 1 {
		t.Fatal("schema-reference tool was not published", err)
	}
	b := s.Tools["demo_tags"]
	for _, input := range []string{`{"tags":["a","b"]}`, `{"tags":null}`, `{}`} {
		if _, err := b.Input.Validate([]byte(input)); err != nil {
			t.Fatalf("valid input rejected: %s: %v", input, err)
		}
	}
	for _, input := range []string{`{"tags":[]}`, `{"tags":["a"]}`, `{"tags":["a","b","c","d"]}`, `{"tags":[1,2]}`} {
		if _, err := b.Input.Validate([]byte(input)); err == nil {
			t.Fatalf("reference or stronger length constraint lost: %s", input)
		}
	}
	if r.Inputs[0].NonEmpty || string(r.Inputs[0].Schema) != string(raw) {
		t.Fatal("source metadata was mutated")
	}
}
