package catalog

import (
	"testing"

	"github.com/openany/openany-mcp/internal/model"
)

func TestExampleQuestionsRequireLiveCatalogEntries(t *testing.T) {
	tools := []struct{ provider, name string }{
		{"scrapecreators", "scrapecreators_get_v1_twitter_user_tweets"},
		{"dataforseo", "dataforseo_post_v3_backlinks_summary_live"},
		{"coingecko", "coingecko_get_api_v3_simple_price"},
		{"qcc", "qcc_post_company_get_company_profile"},
		{"fred", "fred_get_fred_series_observations"},
	}
	var records []model.Record
	for _, tool := range tools {
		record := fixture(tool.name)
		record.APIID = tool.name
		record.ProviderID = tool.provider
		record.Name = tool.name
		record.Description = tool.name
		record.PathTemplate = "/" + tool.name
		record.PublicPath = "/v1/agent-apis/" + tool.provider + "/" + tool.name
		records = append(records, record)
	}
	models := []LLMModel{{ID: "image-1", OutputModalities: []string{"image"}}}
	discovery := newDiscovery(mustBuild(t, records), nil, false, models, true, false)
	overview := discovery.Overview()
	counts := map[string]int{}
	for _, item := range overview["apis_by_category"].([]map[string]any) {
		counts[item["category"].(string)] = item["count"].(int)
	}
	questions := overview["example_questions"].([]map[string]any)
	seen := map[string]bool{}
	for _, item := range questions {
		category := item["category"].(string)
		seen[category] = true
		if len(item["en"].([]string)) == 0 || len(item["zh"].([]string)) == 0 {
			t.Fatal(item)
		}
		if category == "models" {
			if overview["model_count"].(int) < 1 || item["modality"] != "image" {
				t.Fatal(item)
			}
			continue
		}
		if counts[category] < 1 {
			t.Fatal("example category is not in the snapshot", category)
		}
	}
	for _, category := range []string{CategorySocial, CategorySEO, CategoryCrypto, CategoryCompany, CategoryFinance, "models"} {
		if !seen[category] {
			t.Fatal("missing example", category)
		}
	}
	withoutTwitter := records[1:]
	partial := newDiscovery(mustBuild(t, withoutTwitter), nil, false, models, true, false)
	for _, item := range partial.Overview()["example_questions"].([]map[string]any) {
		if item["category"] == CategorySocial {
			t.Fatal("stale social example remained")
		}
	}
}
