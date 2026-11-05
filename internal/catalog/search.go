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
	PricingAvailable bool
	Hash             string
	snapshot         *Snapshot
	entries          map[string]discoveryEntry
}
type discoveryEntry struct {
	fields           [4]string
	summary, version string
	pricing          map[string]any
}
type APISummary struct {
	API      string `json:"api"`
	Title    string `json:"title"`
	Provider string `json:"provider"`
	Summary  string `json:"summary"`
	Method   string `json:"method"`
	Path     string `json:"path"`
}
type SearchResult struct {
	Results    []APISummary `json:"results"`
	Total      int          `json:"total"`
	Ranking    string       `json:"ranking"`
	NextCursor string       `json:"next_cursor,omitempty"`
}
type searchCursor struct {
	Hash, Query, Provider string
	Limit, Offset         int
}

func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func normalizeSearch(s string) string {
	s = strings.ToLower(s)
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
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
func NewDiscovery(s *Snapshot, prices map[string]map[string]any, available bool) *Discovery {
	d := &Discovery{PricingAvailable: available, snapshot: s, entries: map[string]discoveryEntry{}}
	versions := map[string]string{}
	for _, name := range s.Names {
		b := s.Tools[name]
		p := publicPricing(prices[b.APIID])
		if !available {
			p = map[string]any{"status": "unavailable"}
		}
		e := discoveryEntry{summary: short(b.Description, 160), pricing: p}
		e.fields = [4]string{normalizeSearch(name), normalizeSearch(b.Title), normalizeSearch(b.Description), normalizeSearch(b.Request.PublicPath)}
		if props, ok := b.Input.Document["properties"].(map[string]any); ok {
			var names []string
			for n := range props {
				names = append(names, n)
			}
			sort.Strings(names)
			e.fields[3] += " " + normalizeSearch(strings.Join(names, " "))
		}
		e.version = digest([]any{name, b.APIID, b.ProviderID, b.Title, b.Description, b.Input.Document, b.Request, p})
		d.entries[name] = e
		versions[name] = e.version
	}
	d.Hash = digest(versions)
	return d
}
func (d *Discovery) Detail(api string) (map[string]any, bool) {
	e, ok := d.entries[api]
	if !ok {
		return nil, false
	}
	b := d.snapshot.Tools[api]
	return map[string]any{"api": api, "api_id": b.APIID, "title": b.Title, "provider": b.ProviderID, "description": b.Description, "method": b.Request.Method, "public_path": b.Request.PublicPath, "input_schema": b.Input.Document, "pricing": e.pricing, "api_version": e.version}, true
}
func (d *Discovery) Version(api string) string { return d.entries[api].version }
func (d *Discovery) Search(query, provider string, limit int, cursor string) (SearchResult, error) {
	out := SearchResult{Results: []APISummary{}, Ranking: "keyword"}
	if limit == 0 {
		limit = 5
	}
	if limit < 1 || limit > 10 || utf8.RuneCountInString(query) > 256 || len(provider) > 64 {
		return out, fmt.Errorf("query or limit out of range")
	}
	q := normalizeSearch(query)
	if q == "" && provider == "" {
		return out, fmt.Errorf("query or provider is required")
	}
	// A small lexical alias list helps common bilingual queries; this is not semantic search.
	for _, alias := range [][2]string{{"天气", "weather"}, {"推特", "twitter"}, {"推文", "tweet"}, {"搜索", "search"}, {"公司", "company"}, {"股票", "stock"}, {"反向链接", "backlinks"}} {
		q = strings.ReplaceAll(q, alias[0], alias[1])
	}
	words := strings.Fields(q)
	type match struct {
		name  string
		score int
	}
	matches := []match{}
	for _, name := range d.snapshot.Names {
		b := d.snapshot.Tools[name]
		if provider != "" && b.ProviderID != provider {
			continue
		}
		e := d.entries[name]
		score := 0
		found := true
		for _, word := range words {
			part := 0
			for i, field := range e.fields {
				if strings.Contains(field, word) {
					part += []int{8, 6, 2, 1}[i]
				}
			}
			if part == 0 {
				found = false
				break
			}
			score += part
		}
		if !found {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(query), name) {
			score += 10000
		}
		matches = append(matches, match{name, score})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].name < matches[j].name
		}
		return matches[i].score > matches[j].score
	})
	start := 0
	if cursor != "" {
		invalid := fmt.Errorf("invalid or outdated search cursor; restart search")
		if len(cursor) > 2048 {
			return out, invalid
		}
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return out, invalid
		}
		var c searchCursor
		if json.Unmarshal(raw, &c) != nil || c.Hash != d.Hash || c.Query != q || c.Provider != provider || c.Limit != limit || c.Offset < 1 || c.Offset >= len(matches) {
			return out, invalid
		}
		start = c.Offset
	}
	out.Total = len(matches)
	end := min(start+limit, len(matches))
	for _, m := range matches[start:end] {
		b := d.snapshot.Tools[m.name]
		out.Results = append(out.Results, APISummary{m.name, b.Title, b.ProviderID, d.entries[m.name].summary, b.Request.Method, b.Request.PublicPath})
	}
	if end < len(matches) {
		raw, _ := json.Marshal(searchCursor{d.Hash, q, provider, limit, end})
		out.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return out, nil
}
