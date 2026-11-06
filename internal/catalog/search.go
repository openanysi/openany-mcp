package catalog

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Discovery struct {
	PricingAvailable, MetadataAvailable bool
	Hash                                string
	snapshot                            *Snapshot
	entries                             map[string]discoveryEntry
}
type discoveryEntry struct {
	fields           [6]string
	summary, version string
	pricing          map[string]any
	metadata         APIMetadata
	limitations      []string
	required         []string
}
type MatchReason struct {
	Fields []string `json:"fields"`
	Terms  []string `json:"terms"`
}
type APISummary struct {
	API            string          `json:"api"`
	Title          string          `json:"title"`
	Provider       string          `json:"provider"`
	Summary        string          `json:"summary"`
	Method         string          `json:"method"`
	Path           string          `json:"path"`
	Categories     []string        `json:"categories,omitempty"`
	Platform       string          `json:"data_platform,omitempty"`
	Product        string          `json:"data_product,omitempty"`
	Capability     *CapabilityInfo `json:"capability,omitempty"`
	Purpose        string          `json:"purpose"`
	MatchReason    MatchReason     `json:"match_reason"`
	Limitations    []string        `json:"limitations,omitempty"`
	RequiredInputs []string        `json:"required_inputs_brief,omitempty"`
}
type SearchResult struct {
	Results    []APISummary `json:"results"`
	Total      int          `json:"total"`
	Ranking    string       `json:"ranking"`
	NextCursor string       `json:"next_cursor,omitempty"`
}
type SearchQuery struct {
	Query, Provider, Category, Platform, Product, Capability, Cursor string
	Limit                                                            int
}
type searchCursor struct {
	Hash, Query, Provider, Category, Platform, Product, Capability string
	Limit, Offset                                                  int
	Read                                                           bool
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func normalizeSearch(s string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, s)), " ")
}
func short(s string, n int) string {
	r := []rune(strings.TrimSpace(strings.Split(s, "\n")[0]))
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

var aliases = [][2]string{{"企业基本信息", "company profile"}, {"公司基本信息", "company profile"}, {"工商信息", "company profile"}, {"基本资料", "profile"}, {"内部交易", "internal transactions"}, {"兑换报价", "swap quote"}, {"反向链接", "backlinks"}, {"热点新闻", "news"}, {"人工智能", "ai"}, {"以太坊", "ethereum"}, {"天气", "weather"}, {"推特", "twitter"}, {"推文", "tweet"}, {"企业", "company"}, {"公司", "company"}, {"股票", "stock"}, {"地址", "address"}, {"报价", "quote"}, {"新闻", "news"}, {"创建", "create"}, {"删除", "delete"}, {"修改", "update"}, {"转账", "transfer"}, {"执行", "execute"}, {"取消", "cancel"}}

func lexical(s string) string {
	s = strings.ToLower(s)
	for _, a := range aliases {
		s = strings.ReplaceAll(s, a[0], " "+a[1]+" ")
	}
	return normalizeSearch(s)
}
func queryWords(s string) []string {
	for _, p := range []string{"请帮我", "帮我", "查询一下", "查一下", "获取", "查询", "搜索", "检索", "查看", "今天的", "最新的", "今天", "最近", "某个", "某", "的", "一下", "请"} {
		s = strings.ReplaceAll(s, p, " ")
	}
	stop := map[string]bool{"help": true, "me": true, "please": true, "find": true, "get": true, "fetch": true, "look": true, "up": true, "the": true, "a": true, "an": true, "for": true, "of": true, "my": true}
	out := []string{}
	seen := map[string]bool{}
	for _, w := range strings.Fields(lexical(s)) {
		if !stop[w] && !seen[w] {
			out = append(out, w)
			seen[w] = true
		}
	}
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}
func readIntent(q string) bool {
	l := strings.ToLower(q)
	for _, w := range []string{"创建", "删除", "修改", "执行", "提交", "转账", "取消", "create", "delete", "update", "execute", "submit", "transfer", "cancel"} {
		if strings.Contains(l, w) {
			return false
		}
	}
	for _, w := range []string{"查", "搜", "获取", "报价", "查看", "get", "search", "lookup", "quote", "find", "list", "fetch", "read"} {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}
func obviousWrite(b *Binding) bool {
	if b.Request.Method == "DELETE" {
		return true
	}
	if b.Request.Method == "GET" {
		return false
	}
	name := " " + normalizeSearch(b.Request.PublicPath) + " "
	for _, read := range []string{"quote", "search", "lookup", "estimate", "simulate", "check"} {
		if strings.Contains(name, " "+read+" ") {
			return false
		}
	}
	for _, w := range []string{"delete", "remove", "create", "update", "execute", "transfer", "submit", "send", "cancel", "revoke"} {
		if strings.Contains(name, " "+w+" ") {
			return true
		}
	}
	return false
}
func NewDiscovery(s *Snapshot, prices map[string]map[string]any, available bool, metadata ...map[string]APIMetadata) *Discovery {
	d := &Discovery{PricingAvailable: available, snapshot: s, entries: map[string]discoveryEntry{}}
	var metas map[string]APIMetadata
	if len(metadata) > 0 {
		metas = metadata[0]
	}
	versions := map[string]string{}
	index := map[string]any{}
	for _, name := range s.Names {
		b := s.Tools[name]
		m := metas[b.APIID]
		if m.Available != nil && !*m.Available {
			continue
		}
		// Never offer samples from a different observed request-contract revision.
		if m.Revision != 0 && m.Revision != b.Revision {
			m.Samples = nil
		}
		m.Categories = publicCategories(m)
		m.Capability.Name = publicText(m.Capability.Name, 120)
		m.Capability.Description = publicText(m.Capability.Description, 300)
		p := publicPricing(prices[b.APIID])
		if !available {
			p = map[string]any{"status": "unavailable"}
		}
		e := discoveryEntry{summary: short(b.Description, 160), pricing: p, metadata: m}
		if purpose := localized(m.DescriptionTranslations); purpose != "" {
			e.summary = short(purpose, 160)
		} else if m.Capability.Description != "" {
			e.summary = short(m.Capability.Description, 160)
		}
		translations := []string{}
		for _, v := range m.NameTranslations {
			translations = append(translations, publicText(v, 180))
		}
		for _, v := range m.DescriptionTranslations {
			translations = append(translations, publicText(v, 600))
		}
		for _, v := range m.CapabilityTranslations {
			translations = append(translations, publicText(v, 180))
		}
		sort.Strings(translations)
		e.fields = [6]string{lexical(name), lexical(b.Title), lexical(short(b.Description, 600)), lexical(b.Request.PublicPath), lexical(strings.Join(translations, " ")), lexical(strings.Join(m.Categories, " ") + " " + m.Platform + " " + m.Product + " " + m.Capability.Name + " " + m.Capability.Description)}
		if props, ok := b.Input.Document["properties"].(map[string]any); ok {
			var names []string
			for n := range props {
				names = append(names, n)
			}
			sort.Strings(names)
			e.fields[3] += " " + lexical(strings.Join(names, " "))
		}
		if required, ok := b.Input.Document["required"].([]any); ok {
			for _, v := range required {
				if n, ok := v.(string); ok {
					e.required = append(e.required, short(n, 80))
				}
			}
			if len(e.required) > 5 {
				e.required = e.required[:5]
			}
		}
		if b.Request.UnsupportedResponse {
			e.limitations = append(e.limitations, "Response mode is not supported; this API cannot be dispatched.")
		}
		if props, ok := b.Input.Document["properties"].(map[string]any); ok {
			names := []string{}
			for n, v := range props {
				doc, _ := v.(map[string]any)
				description, _ := doc["description"].(string)
				if strings.Contains(description, "serialization policy") {
					names = append(names, n)
				}
			}
			sort.Strings(names)
			for _, n := range names {
				if len(e.limitations) >= 4 {
					break
				}
				e.limitations = append(e.limitations, "Omit optional array parameter "+short(n, 80)+": no serialization policy is configured.")
			}
		}
		e.version = digest([]any{name, b.APIID, b.Revision, b.ProviderID, b.Title, b.Description, b.Input.Document, b.Request, p})
		d.entries[name] = e
		versions[name] = e.version
		index[name] = []any{e.fields, m.Categories, m.Platform, m.Product, m.Capability, e.summary}
	}
	d.Hash = digest([]any{versions, index})
	return d
}
func (d *Discovery) Detail(api string) (map[string]any, bool) {
	e, ok := d.entries[api]
	if !ok {
		return nil, false
	}
	b := d.snapshot.Tools[api]
	return map[string]any{"api": api, "api_id": b.APIID, "title": b.Title, "provider": b.ProviderID, "description": b.Description, "method": b.Request.Method, "public_path": b.Request.PublicPath, "input_schema": b.Input.Document, "pricing": e.pricing, "api_version": e.version, "categories": e.metadata.Categories, "data_platform": e.metadata.Platform, "data_product": e.metadata.Product, "capability": e.metadata.Capability, "limitations": e.limitations}, true
}
func (d *Discovery) Version(api string) string { return d.entries[api].version }
func (d *Discovery) Search(query, provider string, limit int, cursor string) (SearchResult, error) {
	return d.SearchFiltered(SearchQuery{Query: query, Provider: provider, Limit: limit, Cursor: cursor})
}
func (d *Discovery) SearchFiltered(q SearchQuery) (SearchResult, error) {
	out := SearchResult{Results: []APISummary{}, Ranking: "keyword"}
	if q.Limit == 0 {
		q.Limit = 5
	}
	if q.Limit < 1 || q.Limit > 10 || utf8.RuneCountInString(q.Query) > 256 || len(q.Provider) > 64 || len(q.Category) > 128 || len(q.Platform) > 256 || len(q.Product) > 128 || len(q.Capability) > 256 {
		return out, fmt.Errorf("query or filters out of range")
	}
	words := queryWords(q.Query)
	normal := strings.Join(words, " ")
	read := readIntent(q.Query)
	if normal == "" && q.Provider == "" && q.Category == "" && q.Platform == "" && q.Product == "" && q.Capability == "" {
		return out, fmt.Errorf("provide business keywords or a filter")
	}
	type match struct {
		name   string
		score  int
		reason MatchReason
	}
	matches := []match{}
	fieldNames := []string{"api", "title", "description", "path_or_parameter", "translations", "classification_or_capability"}
	weights := []int{8, 6, 2, 1, 7, 6}
	for _, name := range d.snapshot.Names {
		e, exists := d.entries[name]
		if !exists {
			continue
		}
		b := d.snapshot.Tools[name]
		m := e.metadata
		if q.Provider != "" && b.ProviderID != q.Provider || q.Platform != "" && m.Platform != q.Platform || q.Product != "" && m.Product != q.Product || q.Capability != "" && m.Capability.Key != q.Capability && m.Capability.ID != q.Capability {
			continue
		}
		if q.Category != "" {
			found := false
			for _, c := range m.Categories {
				found = found || c == q.Category
			}
			if !found {
				continue
			}
		}
		exact := strings.EqualFold(strings.TrimSpace(q.Query), name)
		if read && !exact && obviousWrite(b) {
			continue
		}
		score := 0
		fields := map[string]bool{}
		terms := []string{}
		for _, word := range words {
			part := 0
			for i, field := range e.fields {
				if strings.Contains(field, word) {
					part += weights[i]
					fields[fieldNames[i]] = true
				}
			}
			if part > 0 {
				score += part
				terms = append(terms, word)
			}
		}
		if len(words) > 0 && score == 0 {
			continue
		}
		if exact {
			score += 10000
		}
		reason := MatchReason{Fields: []string{}, Terms: terms}
		for _, f := range fieldNames {
			if fields[f] {
				reason.Fields = append(reason.Fields, f)
			}
		}
		if len(reason.Terms) > 8 {
			reason.Terms = reason.Terms[:8]
		}
		if len(words) == 0 {
			reason.Fields = []string{"filters"}
		}
		matches = append(matches, match{name, score, reason})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].name < matches[j].name
		}
		return matches[i].score > matches[j].score
	})
	start := 0
	if q.Cursor != "" {
		invalid := fmt.Errorf("invalid or outdated search cursor; restart search")
		if len(q.Cursor) > 4096 {
			return out, invalid
		}
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		if err != nil {
			return out, invalid
		}
		var c searchCursor
		if json.Unmarshal(raw, &c) != nil || c.Hash != d.Hash || c.Query != normal || c.Provider != q.Provider || c.Category != q.Category || c.Platform != q.Platform || c.Product != q.Product || c.Capability != q.Capability || c.Limit != q.Limit || c.Read != read || c.Offset < 1 || c.Offset >= len(matches) {
			return out, invalid
		}
		start = c.Offset
	}
	out.Total = len(matches)
	end := min(start+q.Limit, len(matches))
	for _, v := range matches[start:end] {
		b := d.snapshot.Tools[v.name]
		e := d.entries[v.name]
		m := e.metadata
		var cap *CapabilityInfo
		if m.Capability.ID != "" || m.Capability.Key != "" {
			c := m.Capability
			c.Description = ""
			cap = &c
		}
		out.Results = append(out.Results, APISummary{API: v.name, Title: short(b.Title, 120), Provider: b.ProviderID, Summary: e.summary, Method: b.Request.Method, Path: short(b.Request.PublicPath, 256), Categories: m.Categories, Platform: m.Platform, Product: m.Product, Capability: cap, Purpose: e.summary, MatchReason: v.reason, Limitations: e.limitations, RequiredInputs: e.required})
	}
	if end < len(matches) {
		raw, _ := json.Marshal(searchCursor{Hash: d.Hash, Query: normal, Provider: q.Provider, Category: q.Category, Platform: q.Platform, Product: q.Product, Capability: q.Capability, Limit: q.Limit, Offset: end, Read: read})
		out.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, nil
}
