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

func discoveryFixture(t *testing.T) *Snapshot {
	t.Helper()
	var records []model.Record
	for _, name := range []string{"demo_weather", "demo_forecast", "other_weather"} {
		r := fixture(name)
		r.APIID = name
		r.PathTemplate, r.PublicPath = "/"+name, "/v1/agent-apis/demo/"+name
		r.Name, r.Description = name, "Weather forecasts for cities"
		if strings.HasPrefix(name, "other") {
			r.ProviderID, r.PublicPath = "other", "/v1/agent-apis/other/"+name
		}
		r.Inputs = []model.Input{{Name: "city", Location: "query", Type: "string", Required: true}}
		records = append(records, r)
	}
	s, err := Build(records, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDiscoverySearchRanksBoundsAndBindsCursors(t *testing.T) {
	s := discoveryFixture(t)
	d := NewDiscovery(s, nil, false)
	r, err := d.Search("weather", "demo", 1, "")
	if err != nil || len(r.Results) != 1 || r.Total != 2 || r.NextCursor == "" {
		t.Fatalf("bounded discovery failed: %+v %v", r, err)
	}
	if r.Results[0].API != "demo_weather" {
		t.Fatal("tool-name relevance lost", r.Results)
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "input_schema") || strings.Contains(string(b), "properties") {
		t.Fatal("schema leaked into search")
	}
	page, err := d.Search("weather", "demo", 1, r.NextCursor)
	if err != nil || len(page.Results) != 1 || page.Results[0].API != "demo_forecast" {
		t.Fatal(page, err)
	}
	for _, tc := range []struct {
		q, p  string
		limit int
	}{{"city", "demo", 1}, {"weather", "other", 1}, {"weather", "demo", 2}} {
		if _, err := d.Search(tc.q, tc.p, tc.limit, r.NextCursor); err == nil {
			t.Fatal("cursor crossed query scope")
		}
	}
	if _, err := d.Search("weather", "", 11, ""); err == nil {
		t.Fatal("unbounded search accepted")
	}
	if _, err := d.Search("", "", 0, ""); err == nil {
		t.Fatal("empty unscoped search accepted")
	}
	other := NewDiscovery(s, map[string]map[string]any{"demo_weather": {"mode": "fixed", "currency": "USD", "price_micro_usd": "123", "unit": "request"}}, true)
	if _, err := other.Search("weather", "demo", 1, r.NextCursor); err == nil {
		t.Fatal("outdated cursor accepted")
	}
}

func TestDiscoveryPublicPricingAndVersionsDoNotChangeDirectCatalog(t *testing.T) {
	s := discoveryFixture(t)
	hash := s.Hash
	prices := map[string]map[string]any{"demo_weather": {"mode": "fixed", "currency": "USD", "price_micro_usd": "9007199254740991", "unit": "request", "supplier_price_micro_usd": "private-marker"}}
	a := NewDiscovery(s, prices, true)
	detail, ok := a.Detail("demo_weather")
	if !ok || detail["input_schema"] == nil || detail["api_version"] == "" {
		t.Fatal(detail)
	}
	raw, _ := json.Marshal(detail)
	if strings.Contains(string(raw), "private-marker") || !strings.Contains(string(raw), `"9007199254740991"`) {
		t.Fatal("public pricing lost precision or leaked supplier data", string(raw))
	}
	if _, err := s.Tools["demo_weather"].Input.Validate([]byte(`{}`)); err == nil {
		t.Fatal("required field lost")
	}
	prices["other_weather"] = map[string]any{"mode": "fixed", "currency": "USD", "price_micro_usd": "1", "unit": "request"}
	b := NewDiscovery(s, prices, true)
	bDetail, _ := b.Detail("demo_weather")
	if bDetail["api_version"] != detail["api_version"] {
		t.Fatal("unrelated price invalidated API version")
	}
	prices["demo_weather"]["price_micro_usd"] = "124"
	c := NewDiscovery(s, prices, true)
	cDetail, _ := c.Detail("demo_weather")
	if cDetail["api_version"] == detail["api_version"] || s.Hash != hash {
		t.Fatal("version/hash boundaries violated")
	}
	unavailable := NewDiscovery(s, nil, false)
	u, _ := unavailable.Detail("demo_weather")
	if u["pricing"].(map[string]any)["status"] != "unavailable" {
		t.Fatal("unknown price presented as free", u)
	}
	if _, ok := a.Detail("not_published"); ok {
		t.Fatal("unknown API exposed")
	}
}

func TestAuxiliaryPricingDoesNotBlockCoreReadiness(t *testing.T) {
	r := fixture("demo_list")
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	m := NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		close(entered)
		<-release
		return nil, errors.New("private database error")
	})
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	go func() { done <- m.RefreshPricing(context.Background()) }()
	<-entered
	if !m.Ready() || m.Current().Tools["demo_list"] == nil {
		t.Error("price query blocked original service")
	}
	if m.Status()["lite_ready"] != true {
		t.Error("search unavailable while prices load")
	}
	r.Revision++
	coreDone := make(chan error, 1)
	go func() { coreDone <- m.Refresh(context.Background()) }()
	select {
	case err := <-coreDone:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("slow pricing blocked subsequent core refresh")
	}
	hash := m.Current().Hash
	close(release)
	if err := <-done; err == nil || !m.Ready() || m.Current().Hash != hash || m.Current().Discovery == nil || m.Current().Discovery.PricingAvailable {
		t.Fatal("optional failure poisoned catalog", err, m.Status())
	}
	if m.Status()["lite_ready"] != true || m.Status()["pricing_available"] != false {
		t.Fatal("auxiliary failure not observable", m.Status())
	}
}

func TestLatePricingCannotRepublishAnUnavailableAPI(t *testing.T) {
	a, b := fixture("demo_removed"), fixture("demo_good")
	b.APIID = "other"
	b.PathTemplate = "/other"
	b.PublicPath = "/v1/agent-apis/demo/other"
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	m := NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{a, b}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		close(entered)
		<-release
		return map[string]map[string]any{a.APIID: {"mode": "fixed", "currency": "USD", "price_micro_usd": "10", "unit": "request"}}, nil
	})
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	go func() { done <- m.RefreshPricing(context.Background()) }()
	<-entered
	a.Status = "inactive"
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	hash := m.Current().Hash
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	_, exposed := m.Current().Discovery.Detail("demo_removed")
	if !m.Ready() || m.Current().Hash != hash || m.Current().Tools["demo_removed"] != nil || exposed {
		t.Fatal("late price result resurrected old generation")
	}
}
