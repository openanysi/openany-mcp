package catalog

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/openany/openany-mcp/internal/model"
)

func TestOverviewGroupsLiveSnapshot(t *testing.T) {
	var records []model.Record
	add := func(provider string, n int) {
		for i := 0; i < n; i++ {
			tool := strings.ReplaceAll(provider, "-", "_") + "_item_" + strconv.Itoa(i)
			record := fixture(tool)
			record.APIID = tool
			record.ProviderID = provider
			record.Name = tool
			record.Description = "catalog item"
			record.PathTemplate = "/" + tool
			record.PublicPath = "/v1/agent-apis/" + provider + "/" + tool
			records = append(records, record)
		}
	}
	add("tikhub", 2)
	add("coingecko", 1)
	add("qcc", 1)
	add("dataforseo", 1)
	add("fred", 1)
	add("pkulaw", 1)
	add("wikivoyage", 1)
	add("demo", 1)
	add("context-dev", 1)
	raw := `{
	  "data": [
	    {"id":"deepseek-v4-flash","input_modalities":["chat","coding"],"output_modalities":["chat"],"is_ready":true},
	    {"id":"Hy-Image-3.5-Preview","input_modalities":["chat","image"],"output_modalities":["image"],"supported_features":["image_generation"],"is_ready":true},
	    {"id":"Midjourney-v7","input_modalities":["chat"],"output_modalities":["image"],"supported_features":["image_generation"],"is_ready":true},
	    {"id":"doubao-seed-2-1-pro","input_modalities":["chat","coding"],"output_modalities":["video"],"is_ready":true},
	    {"id":"tts-1","input_modalities":["chat"],"output_modalities":["audio"],"is_ready":true},
	    {"id":"hy-vision-2.0-instruct","input_modalities":["chat","image"],"output_modalities":["chat","image"],"supported_features":["vision"],"is_ready":true},
	    {"id":"embed-small","input_modalities":["chat"],"output_modalities":[],"supported_features":["embedding"],"is_ready":true}
	  ]
	}`
	models, err := ParseModelCatalog([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	discovery := newDiscovery(mustBuild(t, records), nil, false, models, true, false)
	overview := discovery.Overview()
	if testing.Verbose() {
		raw, _ := json.MarshalIndent(overview, "", "  ")
		t.Logf("overview\n%s", raw)
	}
	if overview["api_count"] != 10 || overview["model_count"] != 7 || overview["models_available"] != true || overview["models_stale"] != false {
		t.Fatal(overview["api_count"], overview["model_count"], overview["models_available"], overview["models_stale"])
	}
	providers := overview["apis_by_provider"].([]map[string]any)
	if len(providers) != 9 || providers[0]["provider"] != "tikhub" || providers[0]["count"] != 2 {
		t.Fatal(providers)
	}
	providerSum := 0
	for _, item := range providers {
		providerSum += item["count"].(int)
	}
	if providerSum != 10 {
		t.Fatal(providerSum)
	}
	wantCategory := map[string]int{
		CategorySocial:   2,
		CategoryCrypto:   1,
		CategoryCompany:  2,
		CategorySEO:      1,
		CategoryFinance:  1,
		CategoryLegal:    1,
		CategoryCommerce: 1,
		CategoryOther:    1,
	}
	categories := overview["apis_by_category"].([]map[string]any)
	if len(categories) != len(categoryOrder) {
		t.Fatal(len(categories))
	}
	categorySum := 0
	for i, item := range categories {
		category := item["category"].(string)
		if category != categoryOrder[i] {
			t.Fatal("category order", category, categoryOrder[i])
		}
		count := item["count"].(int)
		if count != wantCategory[category] {
			t.Fatalf("%s count %d", category, count)
		}
		categorySum += count
		names := item["providers"].([]string)
		if category == CategoryOther && (len(names) != 1 || names[0] != "demo") {
			t.Fatal(names)
		}
		if count == 0 && len(names) != 0 {
			t.Fatal("empty category listed providers", category)
		}
	}
	if categorySum != 10 {
		t.Fatal(categorySum)
	}
	company := categories[2]["providers"].([]string)
	if len(company) != 2 || company[0] != "context-dev" || company[1] != "qcc" {
		t.Fatal(company)
	}
	modalities := overview["models_by_modality"].([]map[string]any)
	wantModels := map[string][]string{
		"chat/reasoning": {"deepseek-v4-flash", "hy-vision-2.0-instruct"},
		"image":          {"Hy-Image-3.5-Preview", "Midjourney-v7"},
		"video":          {"doubao-seed-2-1-pro"},
		"audio":          {"tts-1"},
		"other":          {"embed-small"},
	}
	if len(modalities) != 5 {
		t.Fatal(len(modalities), modalities)
	}
	modelSum := 0
	for i, modality := range []string{"chat/reasoning", "image", "video", "audio", "other"} {
		item := modalities[i]
		if item["modality"] != modality {
			t.Fatal(item["modality"])
		}
		ids := item["models"].([]string)
		want := wantModels[modality]
		if item["count"] != len(want) || len(ids) != len(want) {
			t.Fatal(modality, ids)
		}
		for j := range want {
			if ids[j] != want[j] {
				t.Fatal(modality, ids)
			}
		}
		modelSum += item["count"].(int)
	}
	if modelSum != 7 {
		t.Fatal(modelSum)
	}
}

func TestProviderCategoryNormalizesUnknownProviders(t *testing.T) {
	if ProviderCategory("context_dev") != CategoryCompany || ProviderCategory("Context-Dev") != CategoryCompany {
		t.Fatal(ProviderCategory("context_dev"), ProviderCategory("Context-Dev"))
	}
	if ProviderCategory("tikhub") != CategorySocial || ProviderCategory("coingecko-extra") != CategoryCrypto {
		t.Fatal("known provider drifted")
	}
	if ProviderCategory("nws") != CategoryOther || ProviderCategory("typesafe") != CategoryOther || ProviderCategory("brand-new") != CategoryOther {
		t.Fatal("unknown provider was classified")
	}
}
