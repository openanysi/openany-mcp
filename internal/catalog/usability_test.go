package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/model"
)

func TestBusinessSearchUsesPublicMetadataAndBindsEveryFilter(t *testing.T) {
	s := discoveryFixture(t)
	meta := map[string]APIMetadata{}
	for _, name := range s.Names {
		id := s.Tools[name].APIID
		meta[id] = APIMetadata{Categories: []string{"company-data"}, Platform: "Companies", Product: "business", Capability: CapabilityInfo{ID: "cap-1", Key: "company-profile", Name: "企业基本信息"}, NameTranslations: map[string]string{"zh-CN": "企业工商信息查询"}, DescriptionTranslations: map[string]string{"zh-CN": "根据企业名称查询基本资料"}}
	}
	d := NewDiscovery(s, nil, false, meta)
	q := SearchQuery{Query: "请帮我查询某公司的工商信息", Category: "company-data", Platform: "Companies", Product: "business", Capability: "company-profile", Limit: 1}
	r, err := d.SearchFiltered(q)
	if err != nil || len(r.Results) != 1 || r.Total != 3 || r.NextCursor == "" {
		t.Fatal("business-language discovery failed", r, err)
	}
	first := r.Results[0]
	if len(first.Categories) != 1 || first.Purpose == "" || len(first.MatchReason.Fields) == 0 {
		t.Fatal("candidate does not explain fit", first)
	}
	q.Cursor = r.NextCursor
	for _, field := range []string{"category", "platform", "product", "capability"} {
		changed := q
		switch field {
		case "category":
			changed.Category = "other"
		case "platform":
			changed.Platform = "other"
		case "product":
			changed.Product = "other"
		case "capability":
			changed.Capability = "other"
		}
		if _, err := d.SearchFiltered(changed); err == nil {
			t.Error("cursor ignored filter", field)
		}
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "input_schema") {
		t.Fatal("search leaked full schema")
	}
	if d.snapshot.Hash != s.Hash {
		t.Fatal("metadata changed direct catalog hash")
	}
}

func TestReadSearchDoesNotRecommendObviousDeleteOperation(t *testing.T) {
	r := fixture("demo_company_search")
	r.Description = "Company profile lookup"
	r.Name = "查询企业"
	del := r
	name := "demo_company_delete"
	del.ToolName = &name
	del.APIID = "delete"
	del.Method = "DELETE"
	del.PathTemplate = "/delete"
	del.PublicPath = "/v1/agent-apis/demo/delete"
	del.Name = "删除企业"
	s, err := Build([]model.Record{r, del}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDiscovery(s, nil, false)
	result, err := d.Search("帮我查询企业信息", "", 5, "")
	if err != nil || len(result.Results) != 1 || result.Results[0].API != "demo_company_search" {
		t.Fatal("read intent selected wrong operation", result, err)
	}
}

func TestOptionalMetadataFailurePreservesCoreAndReportsUnavailable(t *testing.T) {
	r := fixture("demo_good")
	m := NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return map[string]map[string]any{}, nil }, func(context.Context) (map[string]APIMetadata, error) {
		return nil, errors.New("private metadata failure")
	})
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	hash := m.Current().Hash
	if err := m.RefreshPricing(context.Background()); err != nil {
		t.Fatal("metadata failure broke prices", err)
	}
	if !m.Ready() || m.Current().Hash != hash || m.Status()["metadata_available"] != false {
		t.Fatal("metadata failure poisoned core", m.Status())
	}
}
