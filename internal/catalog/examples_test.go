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
	joined := map[string][]string{}
	for _, item := range questions {
		joined[item["category"].(string)+":en"] = append(joined[item["category"].(string)+":en"], item["en"].([]string)...)
		joined[item["category"].(string)+":zh"] = append(joined[item["category"].(string)+":zh"], item["zh"].([]string)...)
	}
	if !containsAll(joined[CategoryCrypto+":en"], "What is the price of this token?") || !containsAll(joined[CategoryCrypto+":zh"], "这个代币现在什么价？") {
		t.Fatal("token price example missing", joined[CategoryCrypto+":en"])
	}
	if containsAll(joined[CategorySEO+":en"], "Is this keyword worth writing?") || containsAll(joined[CategorySocial+":en"], "How is this being discussed on Xiaohongshu?") || containsAll(joined[CategoryCompany+":en"], "Find emails for this company domain.") {
		t.Fatal("routing example advertised without its tool")
	}
	base := append([]model.Record{}, records...)
	extra := []struct{ provider, name string }{
		{"dataforseo", "dataforseo_post_v3_dataforseo_labs_google_keyword_overview_live"},
		{"justoneapi", "justoneapi_get_api_xiaohongshu_get_note_detail_v1"},
		{"hunter", "hunter_get_v2_email_finder"},
	}
	for _, tool := range extra {
		record := fixture(tool.name)
		record.APIID = tool.name
		record.ProviderID = tool.provider
		record.Name = tool.name
		record.Description = tool.name
		record.PathTemplate = "/" + tool.name
		record.PublicPath = "/v1/agent-apis/" + tool.provider + "/" + tool.name
		records = append(records, record)
	}
	withRouting := newDiscovery(mustBuild(t, records), nil, false, models, true, false)
	prompts := map[string][]string{}
	for _, item := range withRouting.Overview()["example_questions"].([]map[string]any) {
		prompts["en"] = append(prompts["en"], item["en"].([]string)...)
		prompts["zh"] = append(prompts["zh"], item["zh"].([]string)...)
	}
	for _, phrase := range []string{"Is this keyword worth writing?", "How is this being discussed on Xiaohongshu?", "Find emails for this company domain.", "What is the price of this token?", "What is the current price of bitcoin?", "Summarize the backlinks for a domain."} {
		if !containsAll(prompts["en"], phrase) {
			t.Fatal("missing en example", phrase)
		}
	}
	for _, phrase := range []string{"这个关键词在 Google 上值得写吗？", "这个成分在小红书上怎么被种草？", "帮我找这家公司能联系的邮箱。", "这个代币现在什么价？", "比特币现在多少钱？"} {
		if !containsAll(prompts["zh"], phrase) {
			t.Fatal("missing zh example", phrase)
		}
	}
	withoutTwitter := base[1:]
	partial := newDiscovery(mustBuild(t, withoutTwitter), nil, false, models, true, false)
	for _, item := range partial.Overview()["example_questions"].([]map[string]any) {
		if item["category"] == CategorySocial {
			t.Fatal("stale social example remained")
		}
	}
}

func containsAll(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
