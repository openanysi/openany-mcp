package catalog

import "strings"

const (
	CategorySocial   = "social/content"
	CategoryCrypto   = "crypto/on-chain"
	CategoryCompany  = "company & leads"
	CategorySEO      = "SEO/search/scraping"
	CategoryFinance  = "finance/macro"
	CategoryLegal    = "legal"
	CategoryCommerce = "e-commerce/real estate/travel"
	CategoryOther    = "other"
)

// categoryOrder is the stable order openany_overview returns.
var categoryOrder = []string{
	CategorySocial,
	CategoryCrypto,
	CategoryCompany,
	CategorySEO,
	CategoryFinance,
	CategoryLegal,
	CategoryCommerce,
	CategoryOther,
}

// providerCategories maps a provider id to one human category.
// A provider is not split across categories. Multi-vertical scrape hubs
// (anyapi, justoneapi, litescrape) stay in SEO/search/scraping.
// Unlisted providers are "other"; add a row when a provider joins the catalog.
// Keys use hyphens. Lookup also accepts the underscore form used in tool names.
var providerCategories = map[string]string{
	"accurlex":          CategoryLegal,
	"aihot":             CategorySocial,
	"alchemy":           CategoryCrypto,
	"anyapi":            CategorySEO,
	"blockrun":          CategoryCrypto,
	"blockscout":        CategoryCrypto,
	"coingecko":         CategoryCrypto,
	"coingecko-extra":   CategoryCrypto,
	"context-dev":       CategoryCompany,
	"crustdata":         CategoryCompany,
	"dataforseo":        CategorySEO,
	"defillama":         CategoryCrypto,
	"dexscreener":       CategoryCrypto,
	"financialdatasets": CategoryFinance,
	"fred":              CategoryFinance,
	"gang":              CategoryCrypto,
	"geckoterminal":     CategoryCrypto,
	"goplus":            CategoryCrypto,
	"harvestapi":        CategoryCompany,
	"hunter":            CategoryCompany,
	"hyperliquid":       CategoryCrypto,
	"jupiter":           CategoryCrypto,
	"justoneapi":        CategorySEO,
	"keenable":          CategorySEO,
	"lifi":              CategoryCrypto,
	"litescrape":        CategorySEO,
	"pkulaw":            CategoryLegal,
	"polymarket":        CategoryCrypto,
	"qcc":               CategoryCompany,
	"rugcheck":          CategoryCrypto,
	"scrapecreators":    CategorySocial,
	"search1api":        CategorySEO,
	"serper":            CategorySEO,
	"sourcify":          CategoryCrypto,
	"tavily":            CategorySEO,
	"tikhub":            CategorySocial,
	"tinyfish":          CategorySEO,
	"treg":              CategoryCompany,
	"uniswap":           CategoryCrypto,
	"wikimedia":         CategorySocial,
	"wikivoyage":        CategoryCommerce,
	"youcom":            CategorySEO,
	"yuandian":          CategoryCompany,
}

// ProviderCategory returns the human category for a provider id.
// Unknown providers, including weather and one-off utilities, are "other".
func ProviderCategory(provider string) string {
	key := strings.ToLower(strings.TrimSpace(provider))
	key = strings.ReplaceAll(key, "_", "-")
	if category, ok := providerCategories[key]; ok {
		return category
	}
	return CategoryOther
}
