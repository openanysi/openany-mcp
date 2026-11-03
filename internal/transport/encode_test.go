package transport

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/model"
	"net/http"
	"testing"
)

func ptr(s string) *string { return &s }
func TestEncodeRootArrayQueryAndAuthBoundary(t *testing.T) {
	r := model.Request{Method: "POST", PublicPath: "/v1/agent-apis/demo/rpc/{network}", RequestContentType: "application/json", Inputs: []model.Parameter{{Name: "network", Location: "path"}, {Name: "body", Location: "body", Pointer: ptr("")}}}
	e, err := Encode(r, map[string]any{"network": "main", "body": []any{map[string]any{"n": json.Number("9007199254740993")}}})
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != "/v1/agent-apis/demo/rpc/main" || string(e.Body) != `[{"n":9007199254740993}]` {
		t.Fatalf("wrong encoding: %+v %s", e, e.Body)
	}
	if _, err = Encode(r, map[string]any{"network": "../bad", "body": []any{}}); err == nil {
		t.Fatal("unsafe path accepted")
	}
	r.Inputs = []model.Parameter{{Name: "Authorization", Location: "header"}}
	if _, err = Encode(r, map[string]any{"Authorization": "bad"}); err == nil {
		t.Fatal("credential override accepted")
	}
	_ = http.MethodGet
}
func TestEncodeArraysRequireExplicitPolicy(t *testing.T) {
	r := model.Request{Method: "GET", PublicPath: "/v1/agent-apis/demo/list", Inputs: []model.Parameter{{Name: "ticker", Location: "query"}}}
	args := map[string]any{"ticker": []any{"A", "B"}}
	if _, err := Encode(r, args); err == nil {
		t.Fatal("ambiguous array accepted")
	}
	r.QueryArrays = map[string]model.ArrayPolicy{"ticker": {Encoding: "csv", Empty: "omit"}}
	e, err := Encode(r, args)
	if err != nil {
		t.Fatal(err)
	}
	if e.Query != "ticker=A%2CB" {
		t.Fatal(e.Query)
	}
}
func TestBodyAncestorPointersAreRejectedWithoutMutatingInput(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		inputs := []model.Parameter{{Name: "parent", Location: "body", Pointer: ptr("/a")}, {Name: "child", Location: "body", Pointer: ptr("/a/b")}}
		if reverse {
			inputs[0], inputs[1] = inputs[1], inputs[0]
		}
		parent := map[string]any{"x": json.Number("1")}
		r := model.Request{Method: "POST", PublicPath: "/v1/agent-apis/demo/list", RequestContentType: "application/json", Inputs: inputs}
		_, err := Encode(r, map[string]any{"parent": parent, "child": json.Number("2")})
		if err == nil {
			t.Fatal("overlapping body pointers accepted")
		}
		if len(parent) != 1 {
			t.Fatal("validated input mutated")
		}
	}
}
