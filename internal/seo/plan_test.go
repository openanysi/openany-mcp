package seo

import "testing"

func TestNormalizeTarget(t *testing.T) {
	cases := map[string]string{
		"example.com":                     "example.com",
		"WWW.Example.com":                 "example.com",
		"https://www.example.com":         "example.com",
		"https://www.example.com/":        "example.com",
		"blog.example.com":                "blog.example.com",
		"https://blog.example.com":        "blog.example.com",
		"https://example.com/pricing":     "https://example.com/pricing",
		"example.com/pricing":             "https://example.com/pricing",
		"https://blog.example.com/a/b":    "https://blog.example.com/a/b",
		"http://www.example.com/docs/seo": "https://www.example.com/docs/seo",
	}
	for raw, want := range cases {
		got, err := NormalizeTarget(raw)
		if err != nil || got != want {
			t.Fatalf("%s -> %s (%v), want %s", raw, got, err, want)
		}
	}
	if _, err := NormalizeTarget("not a domain"); err == nil {
		t.Fatal("accepted a target with spaces")
	}
}

func TestKeywordResearchPlansOneGoogleCallUntilTrendIsRequested(t *testing.T) {
	plan, err := Build("keyword_research", map[string]any{"keywords": []any{"email finder API"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Calls) != 1 || plan.Calls[0].API != GoogleOverview {
		t.Fatal(plan.Calls)
	}
	task := plan.Calls[0].Tasks[0]
	if task["location_code"] != DefaultLocationCode || task["language_code"] != DefaultLanguageCode {
		t.Fatal("required locale was not defaulted into the unpaid plan", task)
	}
	plan, err = Build("keyword_research", map[string]any{"keywords": []any{"email finder API"}, "include_trend": true, "search_engine": "google"})
	if err != nil || len(plan.Calls) != 2 || plan.Calls[1].API != GoogleHistorical {
		t.Fatal(plan, err)
	}
	plan, err = Build("keyword_research", map[string]any{"keywords": []any{"email finder API"}, "search_engine": "bing"})
	if err != nil || len(plan.Calls) != 1 || plan.Calls[0].API != BingVolume {
		t.Fatal(plan, err)
	}
	if _, err = Build("keyword_research", map[string]any{"keywords": []any{"email finder API"}, "search_engine": "bing", "include_trend": true}); err == nil {
		t.Fatal("bing trend was accepted")
	}
}

func TestKeywordIdeasSeparatesSeedsSiteAndRankings(t *testing.T) {
	ideas, err := Build("keyword_ideas", map[string]any{"seeds": []any{"email finder API", "email finder"}})
	if err != nil || ideas.Calls[0].API != GoogleIdeas {
		t.Fatal(ideas, err)
	}
	related, err := Build("keyword_ideas", map[string]any{"seeds": []any{"email finder API", "ignored"}, "mode": "related", "site": "example.com"})
	if err != nil || related.Calls[0].API != GoogleRelated || related.Calls[0].Tasks[0]["keyword"] != "email finder API" {
		t.Fatal(related, err)
	}
	site, err := Build("keyword_ideas", map[string]any{"site": "https://www.example.com/pricing"})
	if err != nil || site.Calls[0].API != GoogleForSite || site.Calls[0].Tasks[0]["target"] != "https://www.example.com/pricing" {
		t.Fatal(site, err)
	}
	if _, err = Build("keyword_ideas", map[string]any{"site": "example.com", "mode": "related"}); err == nil {
		t.Fatal("related mode accepted a site without a seed")
	}
	if _, err = Build("keyword_ideas", map[string]any{}); err == nil {
		t.Fatal("empty ideas request accepted")
	}
}

func TestRankedKeywordsAndSERPStayOnTheirRoutes(t *testing.T) {
	ranked, err := Build("ranked_keywords", map[string]any{"target": "www.example.com", "positions": 10})
	if err != nil {
		t.Fatal(err)
	}
	task := ranked.Calls[0].Tasks[0]
	if ranked.Calls[0].API != GoogleRanked || task["target"] != "example.com" || task["historical_serp_mode"] != "live" {
		t.Fatal(task)
	}
	filter, _ := task["filters"].([]any)
	if len(filter) != 3 || filter[0] != "ranked_serp_element.serp_item.rank_group" || filter[1] != "<=" || filter[2] != 10 {
		t.Fatal(filter)
	}
	serp, err := Build("serp_overview", map[string]any{"keyword": "email finder API", "search_engine": "bing"})
	if err != nil || serp.Calls[0].API != BingSERP || serp.Calls[0].Tasks[0]["depth"] != PageOneDepth {
		t.Fatal(serp, err)
	}
}

func TestTariffReadsBasePlusReturnedResults(t *testing.T) {
	pricing := map[string]any{
		"status": "available", "mode": "rules", "currency": "USD",
		"policy": map[string]any{"sale": map[string]any{
			"base_price_micro_usd": "12000",
			"max_cost_micro_usd":   "132000",
			"quantities": []any{map[string]any{
				"pointer": "/tasks/0/result/0/items_count", "price_micro_usd": "120",
			}},
		}},
	}
	got := Tariff(pricing, 20)
	if got["is_quote"] != false || got["base_usd"] != "0.012" || got["per_result_usd"] != "0.00012" || got["published_tariff_maximum_usd"] != "0.0144" {
		t.Fatal(got)
	}
	if Tariff(map[string]any{"status": "unavailable"}, 1)["description"] == "" {
		t.Fatal("unavailable price looked readable")
	}
}

func TestPriceVersionChangesWithTheRequest(t *testing.T) {
	left, _ := Build("keyword_research", map[string]any{"keywords": []any{"alpha"}})
	right, _ := Build("keyword_research", map[string]any{"keywords": []any{"beta"}})
	a, err := PriceVersion(left.Calls, []string{"version"})
	if err != nil || len(a) != 64 {
		t.Fatal(a, err)
	}
	b, _ := PriceVersion(left.Calls, []string{"version"})
	c, _ := PriceVersion(right.Calls, []string{"version"})
	d, _ := PriceVersion(left.Calls, []string{"other"})
	if a != b || a == c || a == d {
		t.Fatal("price version did not follow the request and catalog version")
	}
}

func TestProjectKeepsVolumeDifficultyRankAndOrganicURL(t *testing.T) {
	overview, ok := Project(GoogleOverview, map[string]any{"tasks": []any{map[string]any{"result": []any{map[string]any{"items": []any{
		map[string]any{"keyword": "email finder api", "keyword_info": map[string]any{"search_volume": 1000}, "keyword_properties": map[string]any{"keyword_difficulty": 42}},
	}}}}}})
	if !ok {
		t.Fatal("overview not projected")
	}
	row := overview.(map[string]any)["keywords"].([]any)[0].(map[string]any)
	if row["search_volume"] != 1000 || row["keyword_difficulty"] != 42 {
		t.Fatal(row)
	}
	ranked, ok := Project(GoogleRanked, map[string]any{"tasks": []any{map[string]any{"result": []any{map[string]any{"items": []any{
		map[string]any{"keyword_data": map[string]any{"keyword": "email finder api", "keyword_info": map[string]any{"search_volume": 1000}}, "ranked_serp_element": map[string]any{"serp_item": map[string]any{"rank_group": 3, "url": "https://example.com", "title": "Example"}}},
	}}}}}})
	if !ok {
		t.Fatal("ranked not projected")
	}
	rankedRow := ranked.(map[string]any)["ranked_keywords"].([]any)[0].(map[string]any)
	if rankedRow["rank_group"] != 3 || rankedRow["url"] != "https://example.com" {
		t.Fatal(rankedRow)
	}
	history, ok := Project(GoogleHistorical, map[string]any{"tasks": []any{map[string]any{"result": []any{map[string]any{"items": []any{
		map[string]any{"keyword": "email finder api", "history": []any{map[string]any{"year": 2024, "month": 1}}},
	}}}}}})
	if !ok || history.(map[string]any)["keywords"].([]any)[0].(map[string]any)["history"] == nil {
		t.Fatal("historical volume was dropped", history, ok)
	}
	serp, ok := Project(GoogleSERP, map[string]any{"tasks": []any{map[string]any{"result": []any{map[string]any{"items": []any{
		map[string]any{"type": "organic", "rank_group": 1, "title": "Docs", "url": "https://example.com"},
		map[string]any{"type": "people_also_ask", "title": "Questions"},
	}}}}}})
	if !ok || len(serp.(map[string]any)["organic"].([]any)) != 1 {
		t.Fatal(serp, ok)
	}
}
