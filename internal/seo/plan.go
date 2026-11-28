package seo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Call is one DataForSEO task array this tool will send after confirmation.
type Call struct {
	API     string
	Purpose string
	Tasks   []map[string]any
	Units   int
	Note    string
}

// Plan is the unpaid request the price check describes.
type Plan struct {
	Tool     string
	Calls    []Call
	Resolved map[string]any
	Notes    []string
}

type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

func fail(format string, args ...any) error {
	return &InputError{Message: fmt.Sprintf(format, args...)}
}

var languageCodePattern = regexp.MustCompile(`(?i)^[a-z]{2}$`)

// Build plans the DataForSEO calls for one SEO tool. It does not read prices
// or call the gateway.
func Build(tool string, args map[string]any) (*Plan, error) {
	switch tool {
	case "keyword_research":
		return planResearch(args)
	case "keyword_ideas":
		return planIdeas(args)
	case "ranked_keywords":
		return planRanked(args)
	case "serp_overview":
		return planSERP(args)
	default:
		return nil, fail("unknown SEO tool")
	}
}

func planResearch(args map[string]any) (*Plan, error) {
	keywords, err := stringList("keywords", args["keywords"], 1, MaxKeywords)
	if err != nil {
		return nil, err
	}
	engine := enumDefault(args["search_engine"], "google", "google", "bing")
	if engine == "" {
		return nil, fail("search_engine must be google or bing")
	}
	trend := boolArg(args["include_trend"])
	if engine == "bing" && trend {
		return nil, fail("include_trend uses Google historical keyword data and is not available for Bing. Bing v1 returns current search volume only, without keyword difficulty.")
	}
	location, _ := args["location"].(string)
	language, _ := args["language"].(string)
	task := map[string]any{"keywords": keywords}
	defaults, err := applyLocale(task, location, language, true, true)
	if err != nil {
		return nil, err
	}
	plan := &Plan{Tool: "keyword_research", Resolved: map[string]any{"search_engine": engine, "keywords": keywords, "include_trend": trend, "defaults": defaults}}
	if engine == "bing" {
		plan.Calls = []Call{{API: BingVolume, Purpose: "Bing search volume. Keyword difficulty is not available for Bing in v1.", Tasks: []map[string]any{task}, Units: len(keywords)}}
		plan.Notes = append(plan.Notes, "Bing v1 returns search volume only. It does not return keyword difficulty, backlinks, or a SERP.")
		return plan, nil
	}
	plan.Calls = []Call{{API: GoogleOverview, Purpose: "Google search volume and keyword difficulty from DataForSEO Labs Keyword Overview. This is not a list of new keywords.", Tasks: []map[string]any{cloneTask(task)}, Units: len(keywords)}}
	if trend {
		plan.Calls = append(plan.Calls, Call{API: GoogleHistorical, Purpose: "Google volume history for these same keywords since the upstream history window. This is not new-keyword discovery.", Tasks: []map[string]any{cloneTask(task)}, Units: len(keywords)})
		plan.Notes = append(plan.Notes, "include_trend adds Historical Keyword Data for the keywords you sent. A rising trend is not a newly discovered keyword.")
	}
	return plan, nil
}

func planIdeas(args map[string]any) (*Plan, error) {
	seeds, err := stringList("seeds", args["seeds"], 0, MaxSeeds)
	if err != nil {
		return nil, err
	}
	site, _ := args["site"].(string)
	site = strings.TrimSpace(site)
	if len(seeds) == 0 && site == "" {
		return nil, fail("provide seeds or site")
	}
	mode := enumDefault(args["mode"], "ideas", "ideas", "related")
	if mode == "" {
		return nil, fail("mode must be ideas or related")
	}
	limit, err := boundedInt(args["limit"], DefaultLimit, 1, MaxLimit)
	if err != nil {
		return nil, fail("limit must be an integer from 1 to %d", MaxLimit)
	}
	location, _ := args["location"].(string)
	language, _ := args["language"].(string)
	plan := &Plan{Tool: "keyword_ideas", Resolved: map[string]any{"mode": mode, "limit": limit}}
	if len(seeds) > 0 {
		if mode == "related" {
			task := map[string]any{"keyword": seeds[0], "limit": limit}
			defaults, err := applyLocale(task, location, language, true, true)
			if err != nil {
				return nil, err
			}
			note := "Google SERP related searches for the first seed only. Result order is the upstream default."
			if len(seeds) > 1 {
				note += " Later seeds were not sent."
			}
			if site != "" {
				note += " site was not used because seeds were provided."
			}
			plan.Calls = []Call{{API: GoogleRelated, Purpose: "Google related searches for one seed keyword. Not the keywords a domain currently ranks for.", Tasks: []map[string]any{task}, Units: limit, Note: note}}
			plan.Notes = append(plan.Notes, note)
			plan.Resolved["seed"] = seeds[0]
			plan.Resolved["defaults"] = defaults
			return plan, nil
		}
		task := map[string]any{"keywords": seeds, "limit": limit}
		defaults, err := applyLocale(task, location, language, true, false)
		if err != nil {
			return nil, err
		}
		note := "Google Keyword Ideas from the seed terms. Result order is the upstream default, which is relevance."
		if site != "" {
			note += " site was not used because seeds were provided. Pass site alone to get keywords relevant to that site."
		}
		plan.Calls = []Call{{API: GoogleIdeas, Purpose: "Google keyword ideas for an article or landing page. Not a domain's current rankings.", Tasks: []map[string]any{task}, Units: limit, Note: note}}
		plan.Notes = append(plan.Notes, note)
		plan.Resolved["seeds"] = seeds
		plan.Resolved["defaults"] = defaults
		return plan, nil
	}
	if mode == "related" {
		return nil, fail("mode related needs at least one seed. Pass site alone with mode ideas to get keywords relevant to that site, which are not its current rankings.")
	}
	target, err := NormalizeTarget(site)
	if err != nil {
		return nil, fail("site: %s", err.Error())
	}
	task := map[string]any{"target": target, "limit": limit}
	defaults, err := applyLocale(task, location, language, true, false)
	if err != nil {
		return nil, err
	}
	note := "Keywords For Site returns terms relevant to this domain or page. It does not return the keywords that target currently ranks for."
	plan.Calls = []Call{{API: GoogleForSite, Purpose: note, Tasks: []map[string]any{task}, Units: limit, Note: note}}
	plan.Notes = append(plan.Notes, note)
	plan.Resolved["target"] = target
	plan.Resolved["defaults"] = defaults
	return plan, nil
}

func planRanked(args map[string]any) (*Plan, error) {
	raw, _ := args["target"].(string)
	target, err := NormalizeTarget(raw)
	if err != nil {
		return nil, fail("target: %s", err.Error())
	}
	limit, err := boundedInt(args["limit"], DefaultLimit, 1, MaxLimit)
	if err != nil {
		return nil, fail("limit must be an integer from 1 to %d", MaxLimit)
	}
	mode := enumDefault(args["historical_serp_mode"], "live", "live", "lost", "all")
	if mode == "" {
		return nil, fail("historical_serp_mode must be live, lost, or all")
	}
	var positions int
	if _, ok := args["positions"]; ok {
		positions, err = boundedInt(args["positions"], 0, 1, MaxLimit)
		if err != nil || positions < 1 {
			return nil, fail("positions must be an integer from 1 to %d, where 10 means rank_group <= 10", MaxLimit)
		}
	}
	location, _ := args["location"].(string)
	language, _ := args["language"].(string)
	task := map[string]any{"target": target, "limit": limit, "historical_serp_mode": mode}
	defaults, err := applyLocale(task, location, language, false, false)
	if err != nil {
		return nil, err
	}
	if positions > 0 {
		task["filters"] = []any{"ranked_serp_element.serp_item.rank_group", "<=", positions}
	}
	note := "Google keywords this domain, subdomain, or page currently ranks for, with position. Not the highest-volume terms for a topic, and not who ranks for one keyword."
	if mode == "live" {
		note += " live returns current rankings only."
	}
	plan := &Plan{
		Tool:     "ranked_keywords",
		Calls:    []Call{{API: GoogleRanked, Purpose: note, Tasks: []map[string]any{task}, Units: limit, Note: note}},
		Resolved: map[string]any{"target": target, "limit": limit, "historical_serp_mode": mode, "defaults": defaults},
		Notes:    []string{note, "v1 is Google only. Bing ranked keywords are not available."},
	}
	if positions > 0 {
		plan.Resolved["positions"] = positions
	}
	return plan, nil
}

func planSERP(args map[string]any) (*Plan, error) {
	keyword, _ := args["keyword"].(string)
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len([]rune(keyword)) > 700 {
		return nil, fail("keyword is required and must be at most 700 characters")
	}
	engine := enumDefault(args["search_engine"], "google", "google", "bing")
	if engine == "" {
		return nil, fail("search_engine must be google or bing")
	}
	location, _ := args["location"].(string)
	language, _ := args["language"].(string)
	task := map[string]any{"keyword": keyword, "depth": PageOneDepth}
	defaults, err := applyLocale(task, location, language, true, true)
	if err != nil {
		return nil, err
	}
	api := GoogleSERP
	if engine == "bing" {
		api = BingSERP
	}
	note := "Page-1 organic results for this keyword. depth 10 is one SERP page. This is not search volume or keyword difficulty."
	return &Plan{
		Tool:     "serp_overview",
		Calls:    []Call{{API: api, Purpose: note, Tasks: []map[string]any{task}, Units: 1, Note: note}},
		Resolved: map[string]any{"keyword": keyword, "search_engine": engine, "depth": PageOneDepth, "defaults": defaults},
		Notes:    []string{note},
	}, nil
}

func applyLocale(task map[string]any, location, language string, requireLocation, requireLanguage bool) ([]string, error) {
	var defaults []string
	location = strings.TrimSpace(location)
	language = strings.TrimSpace(language)
	if location != "" {
		if code, ok := locationCode(location); ok {
			task["location_code"] = code
		} else if len([]rune(location)) > 120 {
			return nil, fail("location is too long")
		} else {
			task["location_name"] = location
		}
	} else if requireLocation {
		task["location_code"] = DefaultLocationCode
		defaults = append(defaults, "location_code 2840 (United States)")
	}
	if language != "" {
		if languageCodePattern.MatchString(language) {
			task["language_code"] = strings.ToLower(language)
		} else if len([]rune(language)) > 120 {
			return nil, fail("language is too long")
		} else {
			task["language_name"] = language
		}
	} else if requireLanguage {
		task["language_code"] = DefaultLanguageCode
		defaults = append(defaults, "language_code en")
	}
	return defaults, nil
}

func locationCode(s string) (int, bool) {
	if s == "" || len(s) > 10 || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

func stringList(name string, v any, min, max int) ([]string, error) {
	if v == nil {
		if min == 0 {
			return nil, nil
		}
		return nil, fail("provide between %d and %d %s", min, max, name)
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fail("%s must be an array of strings", name)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, fail("%s must be an array of strings", name)
		}
		s = strings.TrimSpace(s)
		if s == "" || len([]rune(s)) > 80 {
			return nil, fail("each %s value must be 1 to 80 characters", name)
		}
		out = append(out, s)
	}
	if len(out) < min || len(out) > max {
		return nil, fail("provide between %d and %d %s", min, max, name)
	}
	return out, nil
}

func enumDefault(v any, fallback string, allowed ...string) string {
	s, _ := v.(string)
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		s = fallback
	}
	for _, item := range allowed {
		if s == item {
			return s
		}
	}
	return ""
}

func boolArg(v any) bool {
	b, _ := v.(bool)
	return b
}

func boundedInt(v any, fallback, min, max int) (int, error) {
	if v == nil {
		return fallback, nil
	}
	var n int
	switch value := v.(type) {
	case int:
		n = value
	case int64:
		n = int(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0, err
		}
		n = int(parsed)
	case float64:
		if value != float64(int(value)) {
			return 0, fmt.Errorf("not an integer")
		}
		n = int(value)
	default:
		return 0, fmt.Errorf("not an integer")
	}
	if n < min || n > max {
		return 0, fmt.Errorf("out of range")
	}
	return n, nil
}

func cloneTask(task map[string]any) map[string]any {
	out := make(map[string]any, len(task))
	for key, value := range task {
		switch item := value.(type) {
		case []string:
			out[key] = append([]string(nil), item...)
		case []any:
			out[key] = append([]any(nil), item...)
		default:
			out[key] = value
		}
	}
	return out
}
