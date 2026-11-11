package catalog

import "strings"

// questionTemplate is a ready-to-ask example. It is returned only when the
// current snapshot still has a tool or model that can serve it.
type questionTemplate struct {
	category string
	provider string
	needles  []string
	modality string
	en       []string
	zh       []string
}

// questionCatalog is checked against the live discovery index. A renamed or
// removed tool drops its question instead of advertising a stale example.
var questionCatalog = []questionTemplate{
	{
		category: CategorySocial,
		provider: "scrapecreators",
		needles:  []string{"twitter_user_tweets"},
		en:       []string{"Show recent tweets from a Twitter user.", "What has this Twitter account posted lately?"},
		zh:       []string{"看某个推特用户最近的推文。", "这个推特账号最近发了什么？"},
	},
	{
		category: CategorySEO,
		provider: "dataforseo",
		needles:  []string{"backlinks_summary"},
		en:       []string{"Summarize the backlinks for a domain.", "Which pages link to this site?"},
		zh:       []string{"查一个域名的反向链接概况。", "哪些页面链接到了这个网站？"},
	},
	{
		category: CategoryCrypto,
		provider: "coingecko",
		needles:  []string{"simple_price"},
		en:       []string{"What is the current price of bitcoin?", "Show the USD price for these coins."},
		zh:       []string{"比特币现在多少钱？", "查这些币的美元价格。"},
	},
	{
		category: CategoryCompany,
		provider: "qcc",
		needles:  []string{"company_profile"},
		en:       []string{"Look up a company's registration profile.", "Find a company by its name."},
		zh:       []string{"查一家公司的工商信息。", "按公司名称找企业。"},
	},
	{
		category: CategoryFinance,
		provider: "fred",
		needles:  []string{"series_observations"},
		en:       []string{"What is the latest CPI reading?", "Show recent observations for a FRED series."},
		zh:       []string{"美国 CPI 最新是多少？", "查一个 FRED 序列最近的数据。"},
	},
	{
		category: "models",
		modality: "image",
		en:       []string{"Draw a simple product photo.", "Generate an image from this description."},
		zh:       []string{"帮我画一张简单的产品图。", "按这段描述生成一张图。"},
	},
}

const overviewNextStep = "If OpenAny has not been introduced yet, answer in the user's language. One sentence: one key covers api_count data APIs and model_count AI models, and browsing and price checks are free and keyless. Then list example_questions for the categories present, with their live counts. Ask what they want to do. Do not call search_apis on a vague first message."

func (d *Discovery) exampleQuestions() []map[string]any {
	out := []map[string]any{}
	for _, spec := range questionCatalog {
		if spec.modality != "" {
			if !d.hasModality(spec.modality) {
				continue
			}
		} else if !d.hasTool(spec.provider, spec.needles) {
			continue
		}
		item := map[string]any{
			"category": spec.category,
			"en":       append([]string(nil), spec.en...),
			"zh":       append([]string(nil), spec.zh...),
		}
		if spec.modality != "" {
			item["modality"] = spec.modality
		}
		out = append(out, item)
	}
	return out
}

func (d *Discovery) hasTool(provider string, needles []string) bool {
	if d == nil || d.snapshot == nil {
		return false
	}
	for _, name := range d.snapshot.Names {
		if _, ok := d.entries[name]; !ok {
			continue
		}
		binding := d.snapshot.Tools[name]
		if binding == nil || !strings.EqualFold(binding.ProviderID, provider) {
			continue
		}
		hay := strings.ToLower(name)
		matched := true
		for _, needle := range needles {
			if !strings.Contains(hay, strings.ToLower(needle)) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func (d *Discovery) hasModality(modality string) bool {
	for _, entry := range d.models {
		if entry.model.primaryModality() == modality {
			return true
		}
	}
	return false
}
