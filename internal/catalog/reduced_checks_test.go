package catalog

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/transport"
)

func TestDescriptionReviewBecomesSafeWarning(t *testing.T) {
	r := fixture("demo_list")
	r.Name = "List internal transactions"
	r.Description = "Internal API key private-marker must not be published"
	r.Inputs = []model.Input{{Name: "limit", Location: "query", Type: "integer", Description: "Authorization private-input-marker"}}
	m := NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, false)
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !m.Ready() || len(m.Current().Tools) != 1 || m.Status()["warning_count"] == nil || m.Status()["warning_count"] == 0 {
		t.Fatal("description review still blocked the API or was not reported", m.Status())
	}
	b := m.Current().Tools["demo_list"]
	if !strings.HasPrefix(b.Description, r.Name) || strings.Contains(b.Description, "private-marker") {
		t.Fatal("unsafe original description was published", b.Description)
	}
	p := b.Input.Document["properties"].(map[string]any)["limit"].(map[string]any)
	if p["description"] != "limit" || r.Inputs[0].Description != "Authorization private-input-marker" {
		t.Fatal("input description was not sanitized immutably", p)
	}
}

func TestCatalogSupportsGatewayMethodsAndBodylessGET(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		for _, contentType := range []string{"none", "application/json"} {
			r := fixture("demo_method")
			r.Method, r.RequestContentType = method, contentType
			if method != "GET" && contentType == "application/json" {
				pointer := ""
				r.Inputs = []model.Input{{Name: "body", Location: "body", Type: "object", Required: true, Pointer: &pointer}}
			}
			s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
			if err != nil || len(s.Tools) != 1 {
				t.Fatalf("%s %s not published: %v %+v", method, contentType, err, s)
			}
			b := s.Tools["demo_method"]
			if b.Request.Method != method {
				t.Fatal("HTTP method changed")
			}
			args := map[string]any{}
			if len(r.Inputs) > 0 {
				args["body"] = map[string]any{"ok": true}
			}
			e, err := transport.Encode(b.Request, args)
			if err != nil {
				t.Fatal(err)
			}
			if method == "GET" || contentType == "none" {
				if len(e.Body) != 0 || e.Headers.Get("Content-Type") != "" || b.Request.RequestContentType != "none" {
					t.Fatal("bodyless request gained a body or content type", e)
				}
			} else if string(e.Body) != `{"ok":true}` {
				t.Fatal("JSON body changed", string(e.Body))
			}
			if r.RequestContentType != contentType {
				t.Fatal("source metadata mutated")
			}
		}
	}
}

func TestCatalogStillRejectsUnencodableBodiesAndMethods(t *testing.T) {
	for _, tc := range []struct{ method, contentType, location string }{
		{"GET", "application/json", "body"}, {"GET", "none", "body"}, {"POST", "none", "body"},
		{"TRACE", "none", ""}, {"POST", "multipart/form-data", "body"},
	} {
		r := fixture("demo_method")
		r.Method, r.RequestContentType = tc.method, tc.contentType
		if tc.location != "" {
			r.Inputs = []model.Input{{Name: "body", Location: tc.location, Type: "object"}}
		}
		s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
		if err != nil || s.Quarantined != 1 || len(s.Tools) != 0 {
			t.Fatalf("unencodable request published: %+v %v", tc, err)
		}
	}
}

func TestMissingOptionalArrayPolicyDoesNotBlockOtherCalls(t *testing.T) {
	r := fixture("demo_optional")
	r.Inputs = []model.Input{{Name: "filters", Location: "query", Type: "array", Nullable: true}}
	s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil || len(s.Tools) != 1 || s.Quarantined != 0 {
		t.Fatal("optional unsupported parameter blocked the API", err, s)
	}
	b := s.Tools["demo_optional"]
	property := b.Input.Document["properties"].(map[string]any)["filters"].(map[string]any)
	if !strings.Contains(property["description"].(string), "serialization policy") {
		t.Fatal("parameter limitation was hidden")
	}
	for _, raw := range []string{`{}`, `{"filters":null}`} {
		args, err := b.Input.Validate([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		e, err := transport.Encode(b.Request, args)
		if err != nil || e.Query != "" {
			t.Fatal("omitted optional parameter could not be called", e, err)
		}
	}
	for _, raw := range []string{`{"filters":[]}`, `{"filters":["a"]}`} {
		args, err := b.Input.Validate([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := transport.Encode(b.Request, args); err == nil {
			t.Fatal("unknown array encoding was guessed")
		}
	}
	r.Inputs[0].Required = true
	r.Inputs[0].Nullable = false
	s, err = Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil || len(s.Tools) != 0 || s.Quarantined != 1 {
		t.Fatal("required unsupported parameter was published", err)
	}
}
