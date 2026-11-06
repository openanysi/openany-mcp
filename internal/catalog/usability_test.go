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

func TestPublicMetadataQueryNeverReadsWholePrivateRecords(t *testing.T) {
	for _, private := range []string{"to_jsonb(a)", "to_jsonb(cap)", "SELECT *", "fixed_headers", "fixed_body", "supplier_policy", "encrypted_key"} {
		if strings.Contains(MetadataQuery, private) {
			t.Fatal("metadata query requires whole-row/private access:", private)
		}
	}
}
func TestSampleOnlyRefreshPublishesGuidanceWithoutInvalidatingExecution(t *testing.T) {
	record := fixture("demo_good")
	label := "first"
	manager := NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return nil, nil }, func(context.Context) (map[string]APIMetadata, error) {
		return map[string]APIMetadata{record.APIID: {Samples: []PublicSample{{Name: label, Request: map[string]any{}}}}}, nil
	})
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := manager.Current()
	label = "second"
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	after := manager.Current()
	if before.Discovery.Hash == after.Discovery.Hash || after.Discovery.entries["demo_good"].metadata.Samples[0].Name != "second" {
		t.Fatal("sample-only update was never published")
	}
	if before.Hash != after.Hash || before.Discovery.Version("demo_good") != after.Discovery.Version("demo_good") {
		t.Fatal("sample display change invalidated execution")
	}
}

func TestSlowMetadataCannotBlockCoreOrOfferOldRevisionSamples(t *testing.T) {
	r := fixture("demo_good")
	r.Revision = 1
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	m := NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) { return nil, nil }, func(context.Context) (map[string]APIMetadata, error) {
		close(entered)
		<-release
		return map[string]APIMetadata{r.APIID: {Revision: 1, Samples: []PublicSample{{Name: "old", Request: map[string]any{}}}}}, nil
	})
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	go func() { done <- m.RefreshPricing(context.Background()) }()
	<-entered
	r.Revision = 2
	coreDone := make(chan error, 1)
	go func() { coreDone <- m.Refresh(context.Background()) }()
	select {
	case err := <-coreDone:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("metadata IO blocked core refresh")
	}
	hash := m.Current().Hash
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	detail, _ := m.Current().Discovery.Detail("demo_good")
	if m.Current().Hash != hash || !m.Ready() || detail["examples"].(map[string]any)["status"] != "stale" {
		t.Fatal("late metadata replaced contract or offered stale example")
	}
}
func TestUnavailableMetadataHidesOnlyLiteCandidate(t *testing.T) {
	s := discoveryFixture(t)
	available := false
	d := NewDiscovery(s, nil, false, map[string]APIMetadata{"demo_weather": {Available: &available}})
	if _, ok := d.Detail("demo_weather"); ok || s.Tools["demo_weather"] == nil {
		t.Fatal("availability affected the wrong view")
	}
}
