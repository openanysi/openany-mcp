package mcpserver

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/gateway"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOutputOptionsValidateBeforeDispatch(t *testing.T) {
	for _, o := range []map[string]any{
		{"mode": "raw", "max_items": json.Number("2")},
		{"mode": "summary", "fields": []any{"id"}},
		{"mode": "compact", "items_path": "rows"},
		{"mode": "compact", "items_path": "/bad~2escape"},
		{"mode": "compact", "max_items": json.Number("1e1000")},
	} {
		if _, err := parseOutput(o); err == nil {
			t.Fatal("accepted invalid presentation", o)
		}
	}
	if _, err := parseOutput(map[string]any{"mode": "compact", "items_path": "/rows", "max_items": json.Number("1e1")}); err != nil {
		t.Fatal(err)
	}
}
func TestOutputProjectionIsExplicitAndPreservesRawByDefault(t *testing.T) {
	data := map[string]any{"rows": []any{map[string]any{"id": json.Number("9007199254740993"), "name": "A", "secret": "supplier value"}, map[string]any{"id": json.Number("2"), "name": "B"}}, "total": json.Number("999")}
	result := gateway.Result{Value: map[string]any{"data": data, "http_status": 200, "request_id": "r"}}
	raw := present(result, outputOptions{})
	if !reflect.DeepEqual(raw.Value["data"], data) || raw.Value["summary"] == nil {
		t.Fatal("default changed raw data")
	}
	opts, _ := parseOutput(map[string]any{"mode": "compact", "items_path": "/rows", "max_items": json.Number("1"), "fields": []any{"id", "name"}})
	projected := present(result, opts)
	rows := projected.Value["data"].(map[string]any)["rows"].([]any)
	if len(rows) != 1 || len(rows[0].(map[string]any)) != 2 || rows[0].(map[string]any)["id"] != json.Number("9007199254740993") || projected.Value["presentation"].(map[string]any)["truncated"] != true {
		t.Fatal(projected)
	}
	if len(data["rows"].([]any)) != 2 || data["rows"].([]any)[0].(map[string]any)["secret"] == nil {
		t.Fatal("mutated upstream result")
	}
	summary := present(result, outputOptions{Mode: "summary"})
	encoded, _ := json.Marshal(summary.Value)
	if summary.Value["data"] != nil || strings.Contains(string(encoded), "supplier value") || summary.Value["summary"] == nil {
		t.Fatal(summary)
	}
	missing := present(result, outputOptions{Mode: "compact", ItemsPath: ptr("/missing"), MaxItems: 1})
	if !missing.IsError || missing.Value["error"].(map[string]any)["execution_state"] != "completed" || missing.Value["request_id"] != "r" {
		t.Fatal(missing)
	}
}
func ptr(s string) *string { return &s }

func TestLiteAddedGuidanceHonorsConfiguredResultBudget(t *testing.T) {
	client, err := gateway.New("http://127.0.0.1:1", time.Second, time.Second, 1024, 1500)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	fields := map[string]any{}
	for i := 0; i < 12; i++ {
		fields[strings.Repeat("x", 79)+strconv.Itoa(i)] = true
	}
	result := present(gateway.Result{Value: map[string]any{"data": fields, "http_status": 200}}, outputOptions{})
	checked := client.LimitResult(result)
	if !checked.IsError || checked.Value["error"].(map[string]any)["execution_state"] != "completed" {
		t.Fatal("oversized result escaped configured limit")
	}
}

func TestTextPresentationKeepsOriginalAndDoesNotInterpretBusiness(t *testing.T) {
	text := "# 今日新闻\n第一条"
	r := gateway.Result{Value: map[string]any{"data": text, "content_type": "text/markdown", "http_status": 200, "request_id": "text-result"}}
	raw := present(r, outputOptions{Mode: "raw"})
	summary := raw.Value["summary"].(map[string]any)
	if raw.Value["data"] != text || summary["type"] != "string" || summary["characters"] != 10 || summary["business_status"] != "not_interpreted" {
		t.Fatal(raw)
	}
	only := present(r, outputOptions{Mode: "summary"})
	if _, ok := only.Value["data"]; ok || only.Value["request_id"] != "text-result" {
		t.Fatal(only)
	}
	compact := withRecovery(present(r, outputOptions{Mode: "compact", Fields: []string{"title"}}))
	if !compact.IsError || compact.Value["data"] != text || compact.Value["error"].(map[string]any)["execution_state"] != "completed" || !strings.Contains(compact.Value["error"].(map[string]any)["next_action"].(string), "do not rerun") {
		t.Fatal(compact)
	}
}
