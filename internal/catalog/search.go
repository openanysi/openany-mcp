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
	ModelsAvailable, ModelsStale        bool
	ModelCount                          int
	Hash                                string
	snapshot                            *Snapshot
	entries                             map[string]discoveryEntry
	models                              map[string]modelEntry
}
type discoveryEntry struct {
	fields           [6]string
	summary, version string
	pricing          map[string]any
	metadata         APIMetadata
	limitations      []string
	required         []string
	samplesStale     bool
}
type MatchReason struct {
	Fields []string `json:"fields"`
	Terms  []string `json:"terms"`
}
type APISummary struct {
	Kind           string          `json:"kind"`
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
	ModelID        string          `json:"model_id,omitempty"`
	Modalities     []string        `json:"modalities,omitempty"`
	ContextLength  int             `json:"context_length,omitempty"`
	PricingBrief   string          `json:"pricing_brief,omitempty"`
}
type SearchResult struct {
	Results    []APISummary `json:"results"`
	Total      int          `json:"total"`
	Ranking    string       `json:"ranking"`
	NextCursor string       `json:"next_cursor,omitempty"`
}
type SearchQuery struct {
	Query, Provider, Category, Platform, Product, Capability, Kind, Cursor string
	Limit                                                                  int
}
type searchCursor struct {
	Hash, Query, Provider, Category, Platform, Product, Capability, Kind string
	RankingVersion                                                       string
	Limit, Offset                                                        int
	Read                                                                 bool
}

const searchRankingVersion = "intent-v4"

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

var aliases = [][2]string{{"企业基本信息", "company profile"}, {"公司基本信息", "company profile"}, {"工商信息", "company profile"}, {"基本资料", "profile"}, {"内部交易", "internal transactions"}, {"兑换报价", "swap quote"}, {"反向链接", "backlinks"}, {"热点新闻", "hot news"}, {"人工智能", "ai"}, {"以太坊", "ethereum"}, {"通义千问", "qwen"}, {"深度求索", "deepseek"}, {"月之暗面", "kimi"}, {"最新的", "latest"}, {"今天的", "latest"}, {"最新", "latest"}, {"今天", "latest"}, {"最近", "latest"}, {"当前", "current"}, {"热点", "hot"}, {"热门", "hot"}, {"历史", "history"}, {"批量", "batch"}, {"详情", "detail"}, {"重置", "reset"}, {"公告", "announcement"}, {"天气", "weather"}, {"推特", "twitter"}, {"推文", "tweet"}, {"企业", "company"}, {"公司", "company"}, {"股票", "stock"}, {"地址", "address"}, {"报价", "quote"}, {"新闻", "news"}, {"创建", "create"}, {"删除", "delete"}, {"修改", "update"}, {"转账", "transfer"}, {"执行", "execute"}, {"取消", "cancel"}, {"写作", "writing"}, {"翻译", "translation"}, {"代码", "code"}, {"看图", "vision"}, {"画图", "imagegen"}, {"生图", "imagegen"}, {"image generation", "imagegen"}, {"对话", "chat"}, {"嵌入", "embedding"}, {"模型", "model"}}

var wordForms = map[string]string{
	"transactions": "transaction", "tokens": "token", "quotes": "quote", "swaps": "swap", "swapping": "swap",
	"resets": "reset", "announcements": "announcement", "topics": "topic", "items": "item", "backlinks": "backlink",
	"historical": "history", "trending": "hot", "bulk": "batch", "details": "detail", "models": "model",
	"translate": "translation", "translating": "translation", "translations": "translation",
	"write": "writing", "writes": "writing", "wrote": "writing",
}

func lexical(s string) string {
	s = strings.ToLower(s)
	for _, a := range aliases {
		s = strings.ReplaceAll(s, a[0], " "+a[1]+" ")
	}
	words := strings.Fields(normalizeSearch(s))
	for i, word := range words {
		if canonical, ok := wordForms[word]; ok {
			words[i] = canonical
		}
	}
	return strings.Join(words, " ")
}
func queryWords(s string) []string {
	for _, p := range []string{"请帮我", "帮我", "查询一下", "查一下", "获取", "查询", "搜索", "检索", "查看", "某个", "某", "的", "一下", "请"} {
		s = strings.ReplaceAll(s, p, " ")
	}
	stop := map[string]bool{"help": true, "me": true, "please": true, "find": true, "get": true, "fetch": true, "look": true, "up": true, "the": true, "a": true, "an": true, "for": true, "of": true, "my": true}
	out := []string{}
	seen := map[string]bool{}
	for _, w := range strings.Fields(lexical(s)) {
		switch w {
		case "recent", "current", "today", "now", "live":
			w = "latest"
		}
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

func modifier(word string) bool {
	return word == "latest" || word == "hot" || word == "history" || word == "batch" || word == "detail"
}

func containsWord(field, word string) bool {
	if strings.IndexFunc(word, func(r rune) bool { return r > unicode.MaxASCII }) >= 0 {
		return strings.Contains(field, word)
	}
	return strings.Contains(" "+field+" ", " "+word+" ")
}

// Exact words beat temporal synonyms; fields are already normalized at refresh time.
func wordMatch(field, word string) int {
	if containsWord(field, word) {
		return 2
	}
	if word == "latest" {
		for _, alternative := range []string{"recent", "current", "today", "now", "live"} {
			if containsWord(field, alternative) {
				return 1
			}
		}
	}
	if word == "news" && containsWord(field, "hot topic") {
		return 1
	}
	return 0
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
	return newDiscovery(s, prices, available, nil, false, false, metadata...)
}
func newDiscovery(s *Snapshot, prices map[string]map[string]any, available bool, models []LLMModel, modelsAvailable, modelsStale bool, metadata ...map[string]APIMetadata) *Discovery {
	d := &Discovery{PricingAvailable: available, ModelsAvailable: modelsAvailable, ModelsStale: modelsStale, snapshot: s, entries: map[string]discoveryEntry{}, models: map[string]modelEntry{}}
	var metas map[string]APIMetadata
	if len(metadata) > 0 {
		metas = metadata[0]
	}
	d.MetadataAvailable = metas != nil
	versions := map[string]string{}
	index := map[string]any{}
	for _, name := range s.Names {
		b := s.Tools[name]
		m := metas[b.APIID]
		if m.Available != nil && !*m.Available {
			continue
		}
		// Never offer samples from a different observed request-contract revision.
		samplesStale := m.Revision != 0 && m.Revision != b.Revision
		if samplesStale {
			m.Samples = nil
		}
		m.Categories = publicCategories(m)
		m.Capability.Name = publicText(m.Capability.Name, 120)
		m.Capability.Description = publicText(m.Capability.Description, 300)
		p := publicPricing(prices[b.APIID])
		if !available {
			p = map[string]any{"status": "unavailable"}
		}
		e := discoveryEntry{summary: short(b.Description, 160), pricing: p, metadata: m, samplesStale: samplesStale}
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
		index[name] = []any{e.fields, m.Categories, m.Platform, m.Product, m.Capability, e.summary, digest(m.Samples), e.samplesStale}
	}
	for _, model := range models {
		if _, exists := d.entries[model.ID]; exists {
			continue
		}
		if _, exists := d.models[model.ID]; exists {
			continue
		}
		d.models[model.ID] = newModelEntry(model)
	}
	d.ModelCount = len(d.models)
	modelIDs := make([]string, 0, len(d.models))
	for id := range d.models {
		modelIDs = append(modelIDs, id)
	}
	sort.Strings(modelIDs)
	modelIndex := []any{}
	for _, id := range modelIDs {
		entry := d.models[id]
		modelIndex = append(modelIndex, []any{id, entry.version, entry.fields, entry.summaryText, entry.pricingBrief})
	}
	d.Hash = digest([]any{searchRankingVersion, versions, index, modelIndex})
	return d
}
func (d *Discovery) Detail(api string) (map[string]any, bool) {
	if entry, ok := d.models[api]; ok {
		detail := entry.detail()
		detail["metadata_available"] = d.MetadataAvailable
		detail["models_available"] = d.ModelsAvailable
		return detail, true
	}
	e, ok := d.entries[api]
	if !ok {
		return nil, false
	}
	b := d.snapshot.Tools[api]
	detail := map[string]any{"api": api, "api_id": b.APIID, "title": b.Title, "provider": b.ProviderID, "description": b.Description, "method": b.Request.Method, "public_path": b.Request.PublicPath, "input_schema": b.Input.Document, "pricing": e.pricing, "api_version": e.version, "categories": e.metadata.Categories, "data_platform": e.metadata.Platform, "data_product": e.metadata.Product, "capability": e.metadata.Capability, "limitations": e.limitations, "metadata_available": d.MetadataAvailable}
	for k, v := range guidance(b, e) {
		detail[k] = v
	}
	detail["kind"] = "api"
	return detail, true
}
func (d *Discovery) Version(api string) string {
	if entry, ok := d.entries[api]; ok {
		return entry.version
	}
	return d.models[api].version
}
func (d *Discovery) Kind(api string) string {
	if _, ok := d.entries[api]; ok {
		return "api"
	}
	if _, ok := d.models[api]; ok {
		return "model"
	}
	return ""
}
func (d *Discovery) Search(query, provider string, limit int, cursor string) (SearchResult, error) {
	return d.SearchFiltered(SearchQuery{Query: query, Provider: provider, Limit: limit, Cursor: cursor})
}
func normalizeKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "all":
		return "all", nil
	case "api", "model":
		return strings.ToLower(strings.TrimSpace(kind)), nil
	default:
		return "", fmt.Errorf("kind must be api, model, or all")
	}
}
func broadTerm(word string) bool {
	switch word {
	case "translation", "writing", "code", "coding", "vision", "image", "chat", "completion", "embedding", "audio", "video", "model", "llm", "ai":
		return true
	default:
		return false
	}
}
func scoreFields(fields [6]string, words, coreWords []string, primaryExtra string) (score, strong, coverage, intent int, reason MatchReason, matched bool) {
	weights := []int{8, 8, 4, 2, 4, 1}
	fieldNames := []string{"api", "title", "description", "path_or_parameter", "translations", "classification_or_capability"}
	if len(words) == 0 {
		return 0, 0, 0, 0, MatchReason{Fields: []string{"filters"}, Terms: []string{}}, true
	}
	fieldsHit := map[string]bool{}
	terms := []string{}
	for _, word := range words {
		part := 0
		primary := wordMatch(primaryExtra, word) > 0
		for i, field := range fields {
			if quality := wordMatch(field, word); quality > 0 {
				part = max(part, weights[i]*quality)
				fieldsHit[fieldNames[i]] = true
				primary = primary || i == 0 || i == 1
			}
		}
		if part > 0 {
			score += part
			terms = append(terms, word)
			if modifier(word) && primary {
				intent++
			} else if !modifier(word) {
				coverage++
				if primary {
					strong++
				}
			}
		}
	}
	if score == 0 || len(coreWords) > 0 && coverage == 0 {
		return score, strong, coverage, intent, MatchReason{}, false
	}
	if len(coreWords) > 1 {
		phrase := strings.Join(coreWords, " ")
		for _, i := range []int{1, 2, 4} {
			if containsWord(fields[i], phrase) {
				score += 4
				break
			}
		}
	}
	reason = MatchReason{Fields: []string{}, Terms: terms}
	for _, name := range fieldNames {
		if fieldsHit[name] {
			reason.Fields = append(reason.Fields, name)
		}
	}
	if len(reason.Terms) > 8 {
		reason.Terms = reason.Terms[:8]
	}
	return score, strong, coverage, intent, reason, true
}
func modelNameHit(fields [6]string, words []string) bool {
	for _, word := range words {
		if broadTerm(word) || modifier(word) {
			continue
		}
		if wordMatch(fields[0], word) > 0 || wordMatch(fields[1], word) > 0 {
			return true
		}
	}
	return false
}
func (d *Discovery) SearchFiltered(q SearchQuery) (SearchResult, error) {
	out := SearchResult{Results: []APISummary{}, Ranking: "keyword"}
	if q.Limit == 0 {
		q.Limit = 5
	}
	kind, kindErr := normalizeKind(q.Kind)
	if kindErr != nil {
		return out, kindErr
	}
	q.Kind = kind
	if q.Limit < 1 || q.Limit > 10 || utf8.RuneCountInString(q.Query) > 256 || len(q.Provider) > 64 || len(q.Category) > 128 || len(q.Platform) > 256 || len(q.Product) > 128 || len(q.Capability) > 256 {
		return out, fmt.Errorf("query or filters out of range")
	}
	words := queryWords(q.Query)
	normal := strings.Join(words, " ")
	read := readIntent(q.Query)
	if normal == "" && q.Provider == "" && q.Category == "" && q.Platform == "" && q.Product == "" && q.Capability == "" && kind != "model" {
		return out, fmt.Errorf("provide business keywords or a filter")
	}
	matches := []searchMatch{}
	coreWords := []string{}
	for _, word := range words {
		if !modifier(word) {
			coreWords = append(coreWords, word)
		}
	}
	if kind != "model" {
		for _, name := range d.snapshot.Names {
			e, exists := d.entries[name]
			if !exists {
				continue
			}
			b := d.snapshot.Tools[name]
			meta := e.metadata
			if q.Provider != "" && b.ProviderID != q.Provider || q.Platform != "" && meta.Platform != q.Platform || q.Product != "" && meta.Product != q.Product || q.Capability != "" && meta.Capability.Key != q.Capability && meta.Capability.ID != q.Capability {
				continue
			}
			if q.Category != "" {
				found := false
				for _, c := range meta.Categories {
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
			score, strong, coverage, intent, reason, matched := scoreFields(e.fields, words, coreWords, lexical(meta.Product+" "+meta.Capability.Name))
			if !exact && !matched {
				continue
			}
			if exact && !matched {
				reason = MatchReason{Fields: []string{"api"}, Terms: []string{}}
			}
			matches = append(matches, searchMatch{name: name, kind: "api", score: score, strong: strong, coverage: coverage, intent: intent, exact: exact, provider: containsWord(normalizeSearch(q.Query), normalizeSearch(b.ProviderID)), reason: reason})
		}
	}
	if kind != "api" {
		ids := make([]string, 0, len(d.models))
		for id := range d.models {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			entry := d.models[id]
			model := entry.model
			if q.Platform != "" || q.Product != "" || q.Provider != "" && !model.matchesProvider(q.Provider) || q.Capability != "" && !model.allowsCapability(q.Capability) || q.Category != "" && !model.matchesCategory(q.Category) {
				continue
			}
			exact := strings.EqualFold(strings.TrimSpace(q.Query), model.ID) || strings.EqualFold(strings.TrimSpace(q.Query), model.displayName())
			score, strong, coverage, intent, reason, matched := scoreFields(entry.fields, words, coreWords, "")
			if !exact && !matched {
				continue
			}
			if exact && !matched {
				reason = MatchReason{Fields: []string{"api"}, Terms: []string{}}
			}
			matches = append(matches, searchMatch{name: id, kind: "model", score: score, strong: strong, coverage: coverage, intent: intent, exact: exact, provider: containsWord(normalizeSearch(q.Query), normalizeSearch(model.Provider)), nameHit: modelNameHit(entry.fields, words), reason: reason})
		}
	}
	better := func(i, j int) bool {
		if matches[i].exact != matches[j].exact {
			return matches[i].exact
		}
		if matches[i].nameHit != matches[j].nameHit {
			return matches[i].nameHit
		}
		if matches[i].provider != matches[j].provider {
			return matches[i].provider
		}
		if matches[i].strong != matches[j].strong {
			return matches[i].strong > matches[j].strong
		}
		if matches[i].intent != matches[j].intent {
			return matches[i].intent > matches[j].intent
		}
		if matches[i].coverage != matches[j].coverage {
			return matches[i].coverage > matches[j].coverage
		}
		if matches[i].score == matches[j].score {
			return matches[i].name < matches[j].name
		}
		return matches[i].score > matches[j].score
	}
	sort.Slice(matches, better)
	hasExact, hasNameHit, hasAPI, hasModel := false, false, false, false
	for _, item := range matches {
		hasExact = hasExact || item.exact
		hasNameHit = hasNameHit || item.nameHit
		hasAPI = hasAPI || item.kind == "api"
		hasModel = hasModel || item.kind == "model"
	}
	if kind == "all" && !hasExact && !hasNameHit && hasAPI && hasModel {
		matches = interleaveKinds(matches)
	}
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
		if json.Unmarshal(raw, &c) != nil || c.RankingVersion != searchRankingVersion || c.Hash != d.Hash || c.Query != normal || c.Provider != q.Provider || c.Category != q.Category || c.Platform != q.Platform || c.Product != q.Product || c.Capability != q.Capability || c.Kind != kind || c.Limit != q.Limit || c.Read != read || c.Offset < 1 || c.Offset >= len(matches) {
			return out, invalid
		}
		start = c.Offset
	}
	out.Total = len(matches)
	end := min(start+q.Limit, len(matches))
	for _, v := range matches[start:end] {
		if v.kind == "model" {
			out.Results = append(out.Results, d.models[v.name].toSummary(v.reason))
			continue
		}
		b := d.snapshot.Tools[v.name]
		e := d.entries[v.name]
		meta := e.metadata
		var cap *CapabilityInfo
		if meta.Capability.ID != "" || meta.Capability.Key != "" {
			c := meta.Capability
			c.Description = ""
			cap = &c
		}
		out.Results = append(out.Results, APISummary{Kind: "api", API: v.name, Title: short(b.Title, 120), Provider: b.ProviderID, Summary: e.summary, Method: b.Request.Method, Path: short(b.Request.PublicPath, 256), Categories: meta.Categories, Platform: meta.Platform, Product: meta.Product, Capability: cap, Purpose: e.summary, MatchReason: v.reason, Limitations: e.limitations, RequiredInputs: e.required})
	}
	if end < len(matches) {
		raw, _ := json.Marshal(searchCursor{Hash: d.Hash, Query: normal, Provider: q.Provider, Category: q.Category, Platform: q.Platform, Product: q.Product, Capability: q.Capability, Kind: kind, Limit: q.Limit, Offset: end, Read: read, RankingVersion: searchRankingVersion})
		out.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, nil
}

type searchMatch struct {
	name, kind                      string
	score, strong, coverage, intent int
	exact, provider, nameHit        bool
	reason                          MatchReason
}

func interleaveKinds(sorted []searchMatch) []searchMatch {
	apis, models := []searchMatch{}, []searchMatch{}
	for _, item := range sorted {
		if item.kind == "model" {
			models = append(models, item)
		} else {
			apis = append(apis, item)
		}
	}
	if len(apis) == 0 || len(models) == 0 {
		return sorted
	}
	startModel := sorted[0].kind == "model"
	out := make([]searchMatch, 0, len(sorted))
	i, j := 0, 0
	modelTurn := startModel
	for i < len(apis) || j < len(models) {
		if modelTurn && j < len(models) {
			out = append(out, models[j])
			j++
			modelTurn = false
			continue
		}
		if i < len(apis) {
			out = append(out, apis[i])
			i++
			modelTurn = true
			continue
		}
		if j < len(models) {
			out = append(out, models[j])
			j++
		}
	}
	return out
}
