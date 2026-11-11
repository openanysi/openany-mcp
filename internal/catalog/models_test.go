package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/model"
)

const modelCatalogFixture = `{
  "data": [
    {"id":"deepseek-v4-flash","name":"deepseek-v4-flash","input_modalities":["chat","coding"],"output_modalities":["chat"],"context_length":1000000,"max_output_length":384000,"pricing":{"prompt":"0.000000225","completion":"0.000000675","image":"0","request":"0","duration":"0","input_cache_read":"0.000000021"},"supported_features":["tool_calling","reasoning"],"is_ready":true,"is_free":false,"openrouter":{"slug":"deepseek-v4-flash"}},
    {"id":"qwen3.7-flash","name":"qwen3.7-flash","input_modalities":["chat"],"output_modalities":["chat"],"context_length":1000000,"pricing":{"prompt":"0.000000023","completion":"0.000000091","image":"0","request":"0","duration":"0","input_cache_read":"0"},"supported_features":["vision","tool_calling"],"is_ready":true,"openrouter":{"slug":"qwen3.7-flash"}},
    {"id":"claude-fable-5.1","name":"Claude Fable","input_modalities":["chat"],"output_modalities":["chat"],"context_length":1000000,"pricing":{"prompt":"0.00001","completion":"0.00005","image":"0","request":"0","duration":"0","input_cache_read":"0"},"supported_features":["tool_calling"],"is_ready":true,"openrouter":{"slug":"claude-fable-5.1"}},
    {"id":"hidden-model","name":"hidden","input_modalities":["chat"],"output_modalities":["chat"],"is_ready":false,"pricing":{"prompt":"1"}},
    {"id":"flux-2-pro","name":"flux-2-pro","input_modalities":["image"],"output_modalities":["image"],"context_length":0,"pricing":{"prompt":"0","completion":"0","image":"0.024","request":"0","duration":"0"},"supported_features":["image_generation"],"is_ready":true,"openrouter":{"slug":"flux-2-pro"}},
    {"id":"qwen3.5-flash","name":"qwen3.5-flash","input_modalities":["chat"],"output_modalities":["chat"],"context_length":1000000,"pricing":[{"range_start":128000,"range_end":256000,"prompt":"0.000000093","completion":"0.000000929","image":"0","request":"0","duration":"0","input_cache_read":"0"},{"range_start":0,"range_end":128000,"prompt":"0.000000023","completion":"0.000000232","image":"0","request":"0","duration":"0","input_cache_read":"0.000000002"}],"is_ready":true,"openrouter":{"slug":"qwen3.5-flash"}},
    {"id":"zero-price","name":"zero-price","input_modalities":["chat"],"output_modalities":["chat"],"context_length":0,"pricing":{"prompt":"0","completion":"0","image":"0","request":"0","duration":"0"},"is_ready":true}
  ]
}`

func TestParseModelCatalogUsesProvidedFieldsOnly(t *testing.T) {
	models, err := ParseModelCatalog([]byte(modelCatalogFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 6 {
		t.Fatalf("hidden model was indexed: %d", len(models))
	}
	byID := map[string]LLMModel{}
	for _, item := range models {
		byID[item.ID] = item
	}
	deepseek := byID["deepseek-v4-flash"]
	if deepseek.Provider != "deepseek" || deepseek.ContextLength != 1000000 || deepseek.pricingBrief() != "input $0.225 per 1M tokens, output $0.675 per 1M tokens, cache read $0.021 per 1M tokens" {
		t.Fatal(deepseek.Provider, deepseek.ContextLength, deepseek.pricingBrief())
	}
	if byID["flux-2-pro"].ContextLength != 0 || byID["flux-2-pro"].pricingBrief() != "image $0.024" {
		t.Fatal(byID["flux-2-pro"].pricingBrief(), byID["flux-2-pro"].ContextLength)
	}
	if brief := byID["qwen3.5-flash"].pricingBrief(); !strings.Contains(brief, "input $0.023 per 1M tokens") || !strings.Contains(brief, "More context tiers") {
		t.Fatal(brief)
	}
	if byID["zero-price"].pricingBrief() != "" || byID["claude-fable-5.1"].Provider != "claude" {
		t.Fatal(byID["zero-price"].pricingBrief(), byID["claude-fable-5.1"].Provider)
	}
}

func TestModelSearchRanksNamesAndMixesTaskWords(t *testing.T) {
	models, err := ParseModelCatalog([]byte(modelCatalogFixture))
	if err != nil {
		t.Fatal(err)
	}
	translate := fixture("demo_translate")
	translate.Name, translate.Description = "Translate text", "Translation between languages"
	translate.PathTemplate, translate.PublicPath = "/translate", "/v1/agent-apis/demo/translate"
	seo := fixture("seo_gpt_page")
	seo.Name, seo.Description = "GPT page scraper", "Scrape pages that mention gpt and deepseek"
	seo.APIID = "seo"
	seo.PathTemplate, seo.PublicPath = "/seo", "/v1/agent-apis/demo/seo"
	plain := NewDiscovery(mustBuild(t, []model.Record{translate, seo}), nil, false)
	withModels := newDiscovery(plain.snapshot, nil, false, models, true, false)
	apiOnly, err := withModels.SearchFiltered(SearchQuery{Query: "translation", Kind: "api"})
	base, errBase := plain.Search("translation", "", 5, "")
	if err != nil || errBase != nil || len(apiOnly.Results) != len(base.Results) {
		t.Fatal(apiOnly, base, err, errBase)
	}
	for i := range base.Results {
		if apiOnly.Results[i].API != base.Results[i].API || apiOnly.Results[i].Kind != "api" || apiOnly.Results[i].Title != base.Results[i].Title {
			t.Fatal("kind=api changed results", apiOnly.Results, base.Results)
		}
	}
	for _, query := range []string{"deepseek", "qwen", "claude", "anthropic", "通义千问"} {
		result, err := withModels.Search(query, "", 5, "")
		if err != nil || len(result.Results) == 0 || result.Results[0].Kind != "model" {
			t.Fatal(query, result, err)
		}
	}
	mixed, err := withModels.Search("翻译", "", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, item := range mixed.Results {
		seen[item.Kind] = true
	}
	if !seen["model"] || !seen["api"] {
		t.Fatal("translation query was not mixed", mixed.Results)
	}
	named, err := withModels.Search("deepseek", "", 5, "")
	if err != nil || named.Results[0].ModelID != "deepseek-v4-flash" || named.Results[0].ContextLength != 1000000 || named.Results[0].PricingBrief == "" {
		t.Fatal(named, err)
	}
	filtered, err := withModels.SearchFiltered(SearchQuery{Category: "llm", Kind: "model", Limit: 10})
	if err != nil || filtered.Total != len(models) {
		t.Fatal(filtered, err)
	}
	chat, err := withModels.SearchFiltered(SearchQuery{Capability: "chat", Kind: "model", Limit: 10})
	if err != nil || chat.Total == 0 {
		t.Fatal(chat, err)
	}
	for _, item := range chat.Results {
		if item.Kind != "model" {
			t.Fatal(item)
		}
	}
	detail, ok := withModels.Detail("claude-fable-5.1")
	schema, _ := detail["input_schema"].(map[string]any)
	if !ok || detail["kind"] != "model" || detail["model_id"] != "claude-fable-5.1" || schema["required"] == nil || detail["pricing_brief"] == "" {
		t.Fatal(detail)
	}
	raw, _ := json.Marshal(detail["pricing"])
	if !strings.Contains(string(raw), "0.00001") {
		t.Fatal("catalog pricing was replaced", string(raw))
	}
	zero, ok := withModels.Detail("zero-price")
	if !ok || zero["pricing_brief"] != nil || zero["context_length"] != nil {
		t.Fatal(zero)
	}
	if _, err := withModels.SearchFiltered(SearchQuery{Kind: "other"}); err == nil {
		t.Fatal("invalid kind accepted")
	}
}

func TestModelCatalogRetainsLastSuccess(t *testing.T) {
	models, err := ParseModelCatalog([]byte(modelCatalogFixture))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	record := fixture("demo_weather")
	manager := NewManagerWithDiscovery(func(context.Context) ([]model.Record, error) { return []model.Record{record}, nil }, model.Overrides{Version: 1}, time.Minute, false, func(context.Context) (map[string]map[string]any, error) {
		return map[string]map[string]any{}, nil
	})
	manager.SetModelLoader(func(context.Context) ([]LLMModel, error) {
		calls++
		if calls > 1 {
			return nil, errors.New("catalog down")
		}
		return models, nil
	})
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	if manager.Status()["models_available"] != true || manager.Status()["models_published"] == 0 {
		t.Fatal(manager.Status())
	}
	core := manager.Current().Hash
	if err := manager.RefreshPricing(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := manager.Current().Discovery.Search("deepseek", "", 5, "")
	if err != nil || len(result.Results) == 0 || result.Results[0].Kind != "model" || manager.Current().Hash != core || manager.Status()["models_stale"] != true {
		t.Fatal(result, err, manager.Status())
	}
}

func TestTextTaskWordsDoNotMatchImageGenerators(t *testing.T) {
	raw := `{
	  "data": [
	    {"id":"deepseek-v4-flash","input_modalities":["chat","coding"],"output_modalities":["chat"],"is_ready":true},
	    {"id":"Hy-Image-3.5-Preview","input_modalities":["chat","image"],"output_modalities":["image"],"supported_features":["image_generation"],"is_ready":true},
	    {"id":"Midjourney-v7","input_modalities":["chat"],"output_modalities":["image"],"supported_features":["image_generation"],"is_ready":true},
	    {"id":"qwen3.7-flash","input_modalities":["chat"],"output_modalities":["chat"],"supported_features":["vision"],"is_ready":true},
	    {"id":"gemini-3.5-flash","input_modalities":["chat","image"],"output_modalities":["chat"],"is_ready":true},
	    {"id":"hy-vision-2.0-instruct","input_modalities":["chat","image"],"output_modalities":["chat","image"],"supported_features":["vision"],"is_ready":true},
	    {"id":"doubao-seed-2-1-pro","input_modalities":["chat","coding"],"output_modalities":["video"],"is_ready":true},
	    {"id":"kimi-k2.7-code","input_modalities":["chat","coding"],"output_modalities":["chat","coding"],"is_ready":true}
	  ]
	}`
	models, err := ParseModelCatalog([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	translate := fixture("demo_translate")
	translate.Name, translate.Description = "Translate text", "Translation between languages"
	translate.PathTemplate, translate.PublicPath = "/translate", "/v1/agent-apis/demo/translate"
	discovery := newDiscovery(mustBuild(t, []model.Record{translate}), nil, false, models, true, false)
	byQuery := func(query string) map[string]bool {
		t.Helper()
		ids := map[string]bool{}
		for _, id := range modelResultIDs(t, discovery, query) {
			ids[id] = true
		}
		return ids
	}
	imageOnly := []string{"Hy-Image-3.5-Preview", "Midjourney-v7", "doubao-seed-2-1-pro"}
	for _, query := range []string{"翻译", "写作", "代码", "translate", "write", "code"} {
		got := byQuery(query)
		for _, id := range imageOnly {
			if got[id] {
				t.Errorf("%s matched %s", query, id)
			}
		}
	}
	translated := byQuery("翻译")
	if !translated["deepseek-v4-flash"] || translated["Hy-Image-3.5-Preview"] || translated["Midjourney-v7"] {
		t.Fatal(translated)
	}
	mixed, err := discovery.Search("翻译", "", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, item := range mixed.Results {
		kinds[item.Kind] = true
		if item.Kind == "model" && (item.ModelID == "Hy-Image-3.5-Preview" || item.ModelID == "Midjourney-v7") {
			t.Fatal("image model interleaved into translation results", item.ModelID)
		}
	}
	if !kinds["model"] || !kinds["api"] {
		t.Fatal(mixed.Results)
	}
	coded := byQuery("代码")
	if !coded["kimi-k2.7-code"] || !coded["deepseek-v4-flash"] || coded["doubao-seed-2-1-pro"] {
		t.Fatal(coded)
	}
	vision := byQuery("看图")
	for _, id := range []string{"qwen3.7-flash", "gemini-3.5-flash", "hy-vision-2.0-instruct"} {
		if !vision[id] {
			t.Errorf("看图 missed %s", id)
		}
	}
	for _, id := range []string{"Hy-Image-3.5-Preview", "Midjourney-v7", "deepseek-v4-flash"} {
		if vision[id] {
			t.Errorf("看图 matched %s", id)
		}
	}
	if !byQuery("vision")["gemini-3.5-flash"] || byQuery("vision")["Midjourney-v7"] {
		t.Fatal(byQuery("vision"))
	}
	for _, query := range []string{"chat", "对话", "completion"} {
		got := byQuery(query)
		for _, id := range imageOnly {
			if got[id] {
				t.Errorf("%s matched %s", query, id)
			}
		}
		for _, id := range []string{"deepseek-v4-flash", "qwen3.7-flash", "gemini-3.5-flash", "kimi-k2.7-code"} {
			if !got[id] {
				t.Errorf("%s missed text model %s", query, id)
			}
		}
	}
	chatRanked, err := discovery.SearchFiltered(SearchQuery{Query: "chat", Kind: "model", Limit: 4})
	if err != nil || chatRanked.Total == 0 {
		t.Fatal(chatRanked, err)
	}
	for _, item := range chatRanked.Results {
		if item.ModelID == "Hy-Image-3.5-Preview" || item.ModelID == "Midjourney-v7" {
			t.Fatal("image model ranked for chat", item.ModelID)
		}
	}
	for _, query := range []string{"画图", "生图", "image generation"} {
		got := byQuery(query)
		for _, id := range []string{"Hy-Image-3.5-Preview", "Midjourney-v7", "hy-vision-2.0-instruct"} {
			if !got[id] {
				t.Errorf("%s missed %s", query, id)
			}
		}
		for _, id := range []string{"qwen3.7-flash", "gemini-3.5-flash", "deepseek-v4-flash"} {
			if got[id] {
				t.Errorf("%s matched text model %s", query, id)
			}
		}
	}
}

func modelResultIDs(t *testing.T, d *Discovery, query string) []string {
	t.Helper()
	q := SearchQuery{Query: query, Kind: "model", Limit: 10}
	var ids []string
	for {
		result, err := d.SearchFiltered(q)
		if err != nil {
			t.Fatal(query, err)
		}
		for _, item := range result.Results {
			ids = append(ids, item.ModelID)
		}
		if result.NextCursor == "" {
			return ids
		}
		q.Cursor = result.NextCursor
	}
}

func TestHTTPModelLoaderReadsCatalogWithoutCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected catalog request %s %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(modelCatalogFixture))
	}))
	defer server.Close()
	models, err := HTTPModelLoader(server.URL, time.Second)(context.Background())
	if err != nil || len(models) != 6 {
		t.Fatal(err, len(models))
	}
}

func TestOptionalLiveModelCatalogFile(t *testing.T) {
	path := os.Getenv("MCP_TEST_MODELS_FILE")
	if path == "" {
		t.Skip()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	models, err := ParseModelCatalog(raw)
	if err != nil || len(models) < 50 {
		t.Fatal(err, len(models))
	}
	empty, err := Build(nil, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	discovery := newDiscovery(empty, nil, false, models, true, false)
	for _, query := range []string{"deepseek", "qwen", "claude", "anthropic", "gpt-4o"} {
		result, err := discovery.SearchFiltered(SearchQuery{Query: query, Kind: "model"})
		if err != nil || result.Total < 1 || result.Results[0].Kind != "model" {
			t.Fatal(query, result, err)
		}
	}
	translate := fixture("demo_translate")
	translate.Name, translate.Description = "Translate text", "Translation between languages"
	translate.PathTemplate, translate.PublicPath = "/translate", "/v1/agent-apis/demo/translate"
	mixedSnapshot := mustBuild(t, []model.Record{translate})
	mixed := newDiscovery(mixedSnapshot, nil, false, models, true, false)
	byID := map[string]LLMModel{}
	for _, item := range models {
		byID[item.ID] = item
	}
	for _, query := range []string{"翻译", "写作", "代码", "translate", "write", "code"} {
		for _, id := range modelResultIDs(t, mixed, query) {
			if !byID[id].textOutput() {
				t.Fatalf("%s matched %s in=%v out=%v", query, id, byID[id].InputModalities, byID[id].OutputModalities)
			}
		}
	}
	for _, id := range []string{"Hy-Image-3.5-Preview", "Midjourney-v7"} {
		for _, query := range []string{"翻译", "看图", "vision"} {
			for _, got := range modelResultIDs(t, mixed, query) {
				if got == id {
					t.Fatalf("%s matched %s", query, id)
				}
			}
		}
	}
	draw := map[string]bool{}
	for _, id := range modelResultIDs(t, mixed, "画图") {
		draw[id] = true
		if !byID[id].hasOutput("image") && !byID[id].hasFeature("image_generation") {
			t.Fatalf("画图 matched %s", id)
		}
	}
	if !draw["Hy-Image-3.5-Preview"] || !draw["Midjourney-v7"] {
		t.Fatal("image generators missing from 画图", len(draw))
	}
	result, err := mixed.Search("翻译", "", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	if testing.Verbose() {
		labels := []string{}
		for _, item := range result.Results {
			labels = append(labels, item.Kind+":"+item.API)
		}
		t.Logf("翻译 page: %s", strings.Join(labels, ", "))
	}
	seen := map[string]bool{}
	for _, item := range result.Results {
		seen[item.Kind] = true
		if item.Kind == "model" && !byID[item.ModelID].textOutput() {
			t.Fatal("image model interleaved", item.ModelID)
		}
	}
	if !seen["model"] || !seen["api"] {
		t.Fatal(result.Results)
	}
}

func mustBuild(t *testing.T, records []model.Record) *Snapshot {
	t.Helper()
	snapshot, err := Build(records, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
