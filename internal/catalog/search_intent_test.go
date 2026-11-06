package catalog

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openany/openany-mcp/internal/model"
)

func intentFixture(t *testing.T) *Discovery {
	t.Helper()
	var records []model.Record
	meta := map[string]APIMetadata{}
	for _, tc := range []struct{ name, title, description, product, category string }{
		{"aihot_agent_codex_resets", "Codex resets as an agent answer", "Announcements and reset credits from the last seven days", "codex-resets", "news"},
		{"aihot_agent_latest", "Latest items as an agent answer", "Recent public AI news, past 24 hours", "news-feeds", "news"},
		{"aihot_agent_hot", "Current hot topics as an agent answer", "Top trending AI news topics", "hot-topics", "news"},
		{"aihot_history", "Historical news archive", "Historical AI news reports", "archive", "news"},
		{"uniswap_quote", "Uniswap get a quote", "Get a swap quote", "swapping", "crypto-defi"},
		{"uniswap_orders", "Uniswap order status", "Read existing swap orders", "swapping", "crypto-defi"},
		{"explorer_address_internal", "List internal transactions involving an address", "Get Ethereum internal transactions for an address", "explorer", "crypto-defi"},
		{"explorer_global_internal", "List internal transactions", "All Ethereum internal transactions", "explorer", "crypto-defi"},
		{"explorer_address_transactions", "List address transactions", "Ethereum address transactions", "explorer", "crypto-defi"},
		{"demo_company_profile", "Company profile", "Company business information", "company-data", "company-data"},
		{"demo_company_delete", "Delete company", "Delete a company record", "company-data", "company-data"},
		{"weather_forecast", "Weather forecast", "Weather for a city", "forecast", "weather"},
	} {
		r := fixture(tc.name)
		r.APIID, r.Name, r.Description = tc.name, tc.title, tc.description
		r.ProviderID = strings.Split(tc.name, "_")[0]
		r.PathTemplate, r.PublicPath = "/"+tc.name, "/v1/agent-apis/"+r.ProviderID+"/"+tc.name
		if strings.HasSuffix(tc.name, "_delete") {
			r.Method = "DELETE"
		}
		records = append(records, r)
		meta[r.APIID] = APIMetadata{Categories: []string{tc.category}, Platform: r.ProviderID, Product: tc.product}
	}
	s, err := Build(records, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	return NewDiscovery(s, nil, false, meta)
}

func TestBusinessIntentSearchCases(t *testing.T) {
	d := intentFixture(t)
	for _, tc := range []struct{ query, want string }{
		{"帮我获取今天的AI热点新闻", "aihot_agent_hot"},
		{"AI热点新闻", "aihot_agent_hot"},
		{"hot AI news", "aihot_agent_hot"},
		{"AI trending news", "aihot_agent_hot"},
		{"帮我获取最新的AI新闻", "aihot_agent_latest"},
		{"今天的AI新闻", "aihot_agent_latest"},
		{"recent AI news", "aihot_agent_latest"},
		{"latest AI news", "aihot_agent_latest"},
		{"历史AI新闻", "aihot_history"},
		{"historical AI news", "aihot_history"},
		{"Codex 重置公告", "aihot_agent_codex_resets"},
		{"Codex reset announcements", "aihot_agent_codex_resets"},
		{"Uniswap 兑换报价", "uniswap_quote"},
		{"帮我获取Uniswap兑换报价", "uniswap_quote"},
		{"Uniswap swap quote", "uniswap_quote"},
		{"Uniswap order status", "uniswap_orders"},
		{"查询以太坊地址的内部交易", "explorer_address_internal"},
		{"Ethereum address internal transactions", "explorer_address_internal"},
		{"Ethereum address transactions", "explorer_address_transactions"},
		{"企业基本信息", "demo_company_profile"},
		{"请帮我查询公司的工商信息", "demo_company_profile"},
		{"删除公司", "demo_company_delete"},
		{"天气", "weather_forecast"},
		{"demo_company_delete", "demo_company_delete"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			r, err := d.Search(tc.query, "", 5, "")
			if err != nil || len(r.Results) == 0 || r.Results[0].API != tc.want {
				t.Fatalf("want top result %s, got %+v, %v", tc.want, r.Results, err)
			}
		})
	}
}

func TestSearchConceptCoverageBeatsDuplicatedMetadata(t *testing.T) {
	a, b := fixture("facts_lookup"), fixture("company_lookup")
	a.APIID, a.Name, a.Description, a.PathTemplate, a.PublicPath = "facts", "Company profile", "Lookup a business record", "/facts", "/v1/agent-apis/demo/facts"
	b.APIID, b.Name, b.Description, b.PathTemplate, b.PublicPath = "company", "Company", "Company", "/company", "/v1/agent-apis/demo/company"
	s, err := Build([]model.Record{a, b}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDiscovery(s, nil, false, map[string]APIMetadata{"company": {Categories: []string{"company"}, Platform: "company", Product: "company", NameTranslations: map[string]string{"en": "company company company"}}})
	r, err := d.Search("company profile", "", 5, "")
	if err != nil || len(r.Results) != 2 || r.Results[0].API != "facts_lookup" {
		t.Fatal("duplicate broad matches outweighed business intent", r, err)
	}
}

func TestLatinSearchDoesNotMatchInsideUnrelatedWords(t *testing.T) {
	r := fixture("chain_details")
	r.APIID, r.Name, r.Description, r.PathTemplate, r.PublicPath = "chain", "Chain information", "Blockchain details", "/chain", "/v1/agent-apis/demo/chain"
	s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDiscovery(s, nil, false)
	result, err := d.Search("ai", "", 5, "")
	if err != nil || result.Total != 0 {
		t.Fatal("AI matched inside chain", result, err)
	}
}

func TestSearchCursorBindsTemporalIntentAndRankingVersion(t *testing.T) {
	d := intentFixture(t)
	r, err := d.Search("今天的新闻", "", 1, "")
	if err != nil || r.NextCursor == "" {
		t.Fatal(r, err)
	}
	if _, err := d.Search("新闻", "", 1, r.NextCursor); err == nil {
		t.Fatal("cursor crossed temporal intent")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(r.NextCursor)
	var cursor map[string]any
	if err := json.Unmarshal(raw, &cursor); err != nil {
		t.Fatal(err)
	}
	cursor["RankingVersion"] = "previous-ranking"
	raw, _ = json.Marshal(cursor)
	if _, err := d.Search("今天的新闻", "", 1, base64.RawURLEncoding.EncodeToString(raw)); err == nil {
		t.Fatal("cursor crossed ranking version")
	}
	filtered, err := d.SearchFiltered(SearchQuery{Query: "news", Provider: "aihot", Category: "news", Product: "news-feeds"})
	if err != nil || filtered.Total != 1 || filtered.Results[0].API != "aihot_agent_latest" {
		t.Fatal("explicit filters changed", filtered, err)
	}
	before := d.snapshot.Hash
	withoutMetadata := NewDiscovery(d.snapshot, nil, false)
	result, err := withoutMetadata.Search("Uniswap quote", "", 5, "")
	if err != nil || result.Results[0].API != "uniswap_quote" || d.snapshot.Hash != before || withoutMetadata.Version("uniswap_quote") != d.Version("uniswap_quote") {
		t.Fatal("ranking depends on metadata or altered execution contracts", result, err)
	}
}

func TestNamedProviderBeatsOtherProvidersExtraKeywordCoverage(t *testing.T) {
	a, b := fixture("uniswap_quote"), fixture("other_swap_quote")
	a.APIID, a.ProviderID, a.Name, a.Description, a.PathTemplate, a.PublicPath = "uniswap", "uniswap", "Uniswap get a quote", "Use the native public HTTP contract", "/quote", "/v1/agent-apis/uniswap/quote"
	b.APIID, b.ProviderID, b.Name, b.Description, b.PathTemplate, b.PublicPath = "other", "other", "Swap quote", "Get a swap quote", "/quote", "/v1/agent-apis/other/quote"
	s, err := Build([]model.Record{a, b}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDiscovery(s, nil, false)
	r, err := d.Search("Uniswap 兑换报价", "", 5, "")
	if err != nil || len(r.Results) != 2 || r.Results[0].Provider != "uniswap" {
		t.Fatal("named supplier was displaced by generic keywords", r, err)
	}
}

func TestNewsDiscoveryDoesNotPromoteStoryCrossReferences(t *testing.T) {
	var records []model.Record
	meta := map[string]APIMetadata{}
	for _, item := range []struct{ name, title, description, product string }{
		{"aihot_story", "AIHOT A story's timeline as an agent answer", "Latest developments, AI summary, only use URLs supplied by the hot answer", "stories"},
		{"aihot_story_digest", "AIHOT Get a story with coverage timeline and AI digest", "Latest news about one story, use IDs from hot topics", "stories"},
		{"aihot_hot", "AIHOT Current hot topics as an agent answer", "Current top ten by rank", "hot-topics"},
		{"aihot_items", "AIHOT List recent public AI items", "Start here for feeds and widgets", "news-feeds"},
	} {
		r := fixture(item.name)
		r.APIID, r.Name, r.Description, r.PathTemplate, r.PublicPath = item.name, item.title, item.description, "/"+item.name, "/v1/agent-apis/demo/"+item.name
		records = append(records, r)
		meta[r.APIID] = APIMetadata{Categories: []string{"news"}, Product: item.product}
	}
	s, err := Build(records, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []map[string]APIMetadata{nil, meta} {
		d := NewDiscovery(s, nil, false, metadata)
		r, err := d.Search("帮我获取今天的AI热点新闻", "", 5, "")
		if err != nil || len(r.Results) == 0 || strings.Contains(r.Results[0].API, "story") {
			t.Fatal("a story detail displaced general news discovery", r, err)
		}
	}
}
