package seo

// Catalog tool names for the DataForSEO routes this server calls.
// Ranked keywords and keyword ideas are Google Labs only.
// Keyword difficulty for Bing is not in this set: Bing v1 returns search volume only.
const (
	GoogleOverview   = "dataforseo_post_v3_dataforseo_labs_google_keyword_overview_live"
	GoogleHistorical = "dataforseo_post_v3_dataforseo_labs_google_historical_keyword_dat"
	GoogleIdeas      = "dataforseo_post_v3_dataforseo_labs_google_keyword_ideas_live"
	GoogleRelated    = "dataforseo_post_v3_dataforseo_labs_google_related_keywords_live"
	GoogleForSite    = "dataforseo_post_v3_dataforseo_labs_google_keywords_for_site_live"
	GoogleRanked     = "dataforseo_post_v3_dataforseo_labs_google_ranked_keywords_live"
	BingVolume       = "dataforseo_post_v3_keywords_data_bing_search_volume_live"
	GoogleSERP       = "dataforseo_post_v3_serp_google_organic_live_regular"
	BingSERP         = "dataforseo_post_v3_serp_bing_organic_live_regular"
)

const (
	DefaultLocationCode = 2840
	DefaultLanguageCode = "en"
	DefaultLimit        = 20
	MaxLimit            = 100
	MaxKeywords         = 100
	MaxSeeds            = 20
	PageOneDepth        = 10
)
