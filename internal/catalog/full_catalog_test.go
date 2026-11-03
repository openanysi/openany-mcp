package catalog

import (
	"encoding/json"
	"github.com/openany/openany-mcp/internal/model"
	"os"
	"testing"
)

func TestFullCatalogSnapshot(t *testing.T) {
	path := os.Getenv("MCP_TEST_CATALOG_FILE")
	if path == "" {
		t.Skip("set MCP_TEST_CATALOG_FILE for full metadata validation")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var records []model.Record
	if err = json.Unmarshal(raw, &records); err != nil {
		t.Fatal(err)
	}
	o := model.Overrides{Version: 1, APIs: map[string]model.Override{}}
	if p := os.Getenv("MCP_TEST_OVERRIDES_FILE"); p != "" {
		raw, err = os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &o); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range records {
		if r.Status != "active" {
			continue
		}
		_, err := Build([]model.Record{r}, o, true)
		if err != nil {
			t.Errorf("%s %s: %v", r.ProviderID, r.PathTemplate, err)
		}
	}
}
