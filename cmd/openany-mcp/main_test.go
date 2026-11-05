package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/catalog"
	"github.com/openany/openany-mcp/internal/model"
)

func diagnosticManager(t *testing.T, hardError bool) *catalog.Manager {
	t.Helper()
	name := "demo_list"
	r := model.Record{APIID: "api-1", ProviderID: "demo", Name: "List", ToolName: &name, Status: "active", Method: "GET", PathTemplate: "/list", PublicPath: "/v1/agent-apis/demo/list", RequestContentType: "none", ConfigPresent: true, UnknownParameters: "reject", Description: "Internal private-description-marker"}
	if hardError {
		r.Method = "TRACE"
	}
	m := catalog.NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, true)
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCatalogReportOnlyFailsForHardIssues(t *testing.T) {
	for _, hard := range []bool{false, true} {
		m := diagnosticManager(t, hard)
		var out bytes.Buffer
		code := writeCatalogReport(&out, m)
		if (code != 0) != hard {
			t.Fatal("warnings changed catalog check exit status", code)
		}
		var report map[string]any
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		key := "warnings"
		if hard {
			key = "issues"
		}
		issue := report[key].([]any)[0].(map[string]any)
		if issue["api_id"] != "api-1" || issue["provider_id"] != "demo" || issue["path_template"] != "/list" || issue["request_content_type"] != "none" {
			t.Fatal("diagnostics lost API identity", issue)
		}
		if strings.Contains(out.String(), "private-description-marker") {
			t.Fatal("raw description leaked into diagnostics")
		}
	}
}

func TestCatalogDiagnosticsLogOncePerGeneration(t *testing.T) {
	m := diagnosticManager(t, false)
	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, nil))
	hash := logCatalogDiagnostics(logger, m.Current(), "")
	first := out.String()
	if !strings.Contains(first, `"api_id":"api-1"`) || !strings.Contains(first, `"provider_id":"demo"`) || strings.Contains(first, "private-description-marker") {
		t.Fatal("unsafe or incomplete warning log", first)
	}
	if got := logCatalogDiagnostics(logger, m.Current(), hash); got != hash || out.String() != first {
		t.Fatal("unchanged diagnostics were logged repeatedly")
	}
	logCatalogDiagnostics(logger, diagnosticManager(t, true).Current(), hash)
	if !strings.Contains(out.String(), "catalog API quarantined") {
		t.Fatal("new generation did not log quarantined identity")
	}
}
