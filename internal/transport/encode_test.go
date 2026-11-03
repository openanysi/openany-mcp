package transport

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/model"
	"net/http"
	"testing"
)

func ptr(s string) *string { return &s }
func TestEncodeRootArrayQueryAndAuthBoundary(t *testing.T) {
	r := model.Record{ProviderID: "demo", Method: "POST", PathTemplate: "/rpc/{network}", PublicPath: "/v1/agent-apis/demo/rpc/{network}", RequestContentType: "application/json", Inputs: []model.Input{{Name: "network", Location: "path", Type: "string"}, {Name: "body", Location: "body", Pointer: ptr(""), Type: "array"}}}
	e, err := Encode(r, model.Override{}, map[string]any{"network": "main", "body": []any{map[string]any{"n": json.Number("9007199254740993")}}})
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != "/v1/agent-apis/demo/rpc/main" || string(e.Body) != `[{"n":9007199254740993}]` {
		t.Fatalf("wrong encoding: %+v %s", e, e.Body)
	}
	if _, err = Encode(r, model.Override{}, map[string]any{"network": "../bad", "body": []any{}}); err == nil {
		t.Fatal("unsafe path accepted")
	}
	r.Inputs = []model.Input{{Name: "Authorization", Location: "header", Type: "string"}}
	if _, err = Encode(r, model.Override{}, map[string]any{"Authorization": "bad"}); err == nil {
		t.Fatal("credential override accepted")
	}
	_ = http.MethodGet
}
func TestEncodeArraysRequireExplicitPolicy(t *testing.T) {
	r := model.Record{Method: "GET", PublicPath: "/v1/agent-apis/demo/list", Inputs: []model.Input{{Name: "ticker", Location: "query", Type: "array"}}}
	args := map[string]any{"ticker": []any{"A", "B"}}
	if _, err := Encode(r, model.Override{}, args); err == nil {
		t.Fatal("ambiguous array accepted")
	}
	e, err := Encode(r, model.Override{QueryArrays: map[string]model.ArrayEncoding{"ticker": {Encoding: "csv", Empty: "omit"}}}, args)
	if err != nil {
		t.Fatal(err)
	}
	if e.Query != "ticker=A%2CB" {
		t.Fatal(e.Query)
	}
}
