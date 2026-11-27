package gateway

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var candidateHash = regexp.MustCompile(`^[a-f0-9]{64}$`)
var candidateUUID = regexp.MustCompile(`^[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}$`)
var candidateAmount = regexp.MustCompile(`^(0|[1-9][0-9]{0,15})(\.[0-9]{1,12})?$`)

type candidateDifference struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type candidateMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type candidateCompatibility struct {
	Level        string                `json:"level"`
	Differences  []candidateDifference `json:"differences"`
	InputMapping []candidateMapping    `json:"input_mapping"`
}
type candidateExecution struct {
	Protocol       string         `json:"protocol"`
	Method         string         `json:"method"`
	Path           string         `json:"path"`
	DefinitionPath string         `json:"definition_path"`
	Overrides      map[string]any `json:"request_overrides"`
}
type publicCandidate struct {
	CandidateID   string                 `json:"candidate_id"`
	Kind          string                 `json:"kind"`
	Relation      string                 `json:"relation"`
	Operation     string                 `json:"operation_key"`
	Provider      string                 `json:"provider_id"`
	Name          string                 `json:"name"`
	APIID         string                 `json:"api_id,omitempty"`
	Revision      int                    `json:"api_revision,omitempty"`
	ModelID       string                 `json:"model_id,omitempty"`
	ModelRevision string                 `json:"model_revision,omitempty"`
	Method        string                 `json:"method"`
	Path          string                 `json:"public_path"`
	Definition    string                 `json:"definition_path"`
	Price         string                 `json:"price_micro_usd,omitempty"`
	Currency      string                 `json:"currency"`
	Unit          string                 `json:"billing_unit"`
	Compatibility candidateCompatibility `json:"compatibility"`
	Execution     candidateExecution     `json:"execution"`
	Pricing       map[string]any         `json:"pricing"`
	Availability  map[string]any         `json:"availability"`
	Checks        []string               `json:"checks"`
}

func boundedText(s string, n int) bool {
	return utf8.RuneCountInString(s) <= n && strings.IndexFunc(s, unicode.IsControl) < 0
}
func candidatePath(s string) bool {
	if !boundedText(s, 1024) || !strings.HasPrefix(s, "/") || strings.HasPrefix(s, "//") {
		return false
	}
	for i := 0; i < 5; i++ {
		if strings.ContainsAny(s, "?#\\ \t\r\n") {
			return false
		}
		for _, v := range strings.Split(s, "/") {
			if v == "." || v == ".." {
				return false
			}
		}
		next, e := url.PathUnescape(s)
		if e != nil {
			return false
		}
		if next == s {
			return true
		}
		s = next
	}
	return false
}
func definitionPath(s, kind, id string) bool {
	if kind == "api" {
		return candidateUUID.MatchString(id) && s == "/api/v1/agent-apis/"+id
	}
	u, e := url.Parse(s)
	if e != nil || u.IsAbs() || u.Host != "" || u.Fragment != "" || u.Path != "/api/v1/key-market/model-prices" {
		return false
	}
	q, e := url.ParseQuery(u.RawQuery)
	return e == nil && len(q) == 1 && len(q["modelid"]) == 1 && q.Get("modelid") == id
}
func officialDefinitionTarget(s string) bool {
	if strings.HasPrefix(s, "/api/v1/agent-apis/") {
		return definitionPath(s, "api", strings.TrimPrefix(s, "/api/v1/agent-apis/"))
	}
	u, e := url.Parse(s)
	return e == nil && definitionPath(s, "model", u.Query().Get("modelid")) && u.Query().Get("modelid") != ""
}
func candidateProviderCode(code string) bool {
	switch code {
	case "openany_upstream_service_error", "openany_upstream_rate_limited", "openany_upstream_unavailable", "openany_upstream_model_not_available", "openany_no_upstream_available", "openany_model_not_available", "openany_route_unavailable":
		return true
	}
	return false
}
func mappingReference(s string) bool {
	location, name, ok := strings.Cut(s, ":")
	if !ok || name == "" || !boundedText(s, 256) || (location != "path" && location != "query" && location != "body") {
		return false
	}
	lower := strings.ToLower(name)
	for _, v := range []string{"authorization", "api_key", "apikey", "token", "password", "secret"} {
		if strings.Contains(lower, v) {
			return false
		}
	}
	return true
}
func boundedCandidate(raw any) (map[string]any, bool) {
	data, e := json.Marshal(raw)
	if e != nil || len(data) > 4096 {
		return nil, false
	}
	var c publicCandidate
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if decoder.Decode(&c) != nil {
		return nil, false
	}
	if !candidateHash.MatchString(c.CandidateID) || !publicToken.MatchString(c.Provider) || !publicToken.MatchString(c.Operation) || c.Name == "" || !boundedText(c.Name, 128) || !candidatePath(c.Path) || (c.Method != "GET" && c.Method != "POST") || c.Currency != "USD" {
		return nil, false
	}
	if c.Execution.Path != c.Path || c.Execution.Method != c.Method || c.Execution.DefinitionPath != c.Definition || c.Execution.Overrides == nil {
		return nil, false
	}
	k := c.Compatibility
	if (k.Level != "direct" && k.Level != "requires_adaptation" && k.Level != "similar") || len(k.Differences) > 8 || len(k.InputMapping) > 32 {
		return nil, false
	}
	for _, d := range k.Differences {
		if !publicToken.MatchString(d.Code) || !boundedText(d.Message, 256) || strings.Contains(strings.ToLower(d.Message), "http") || strings.Contains(strings.ToLower(d.Message), "bearer") {
			return nil, false
		}
	}
	for _, m := range k.InputMapping {
		if !mappingReference(m.From) || !mappingReference(m.To) {
			return nil, false
		}
	}
	if c.Availability["published"] != true {
		return nil, false
	}
	health, _ := c.Availability["health"].(string)
	if health != "unknown" && health != "observed_available" && health != "recent_failure" {
		return nil, false
	}
	c.Availability = map[string]any{"published": true, "health": health}
	if c.Kind == "api" {
		if !candidateUUID.MatchString(c.APIID) || c.Revision < 1 || c.Relation != "same_operation" || c.Execution.Protocol != "http" || !strings.HasPrefix(c.Path, "/v1/agent-apis/"+c.Provider+"/") || !definitionPath(c.Definition, "api", c.APIID) || !candidateAmount.MatchString(c.Price) || c.Price != c.Pricing["amount_micro_usd"] || len(c.Execution.Overrides) != 0 || c.ModelID != "" {
			return nil, false
		}
	} else if c.Kind == "model" {
		protocols := map[string]string{"openai-chat-completions": "/v1/chat/completions", "openai-responses": "/v1/responses", "anthropic-messages": "/v1/messages", "openai-images": "/v1/images/", "openai-video": "/v1/videos", "openai-audio": "/v1/audio/"}
		_, ok := protocols[c.Execution.Protocol]
		allowed := map[string][]string{"openai-chat-completions": {"/v1/chat/completions"}, "openai-responses": {"/v1/responses"}, "anthropic-messages": {"/v1/messages"}, "openai-images": {"/v1/images/generations", "/v1/images/edits"}, "openai-video": {"/v1/videos"}, "openai-audio": {"/v1/audio/speech", "/v1/audio/transcriptions"}}
		matched := false
		for _, p := range allowed[c.Execution.Protocol] {
			if p == c.Path {
				matched = true
			}
		}
		if !ok || !matched || c.Method != "POST" || c.ModelID == "" || !boundedText(c.ModelID, 128) || !candidateHash.MatchString(c.ModelRevision) || !definitionPath(c.Definition, "model", c.ModelID) || (c.Relation != "same_model" && c.Relation != "similar_model") || c.APIID != "" {
			return nil, false
		}
		route, ok := c.Execution.Overrides["route"].(map[string]any)
		if !ok || len(route) != 1 || route["provider"] != c.Provider || c.Execution.Overrides["model"] != c.ModelID {
			return nil, false
		}
		for key, v := range c.Execution.Overrides {
			switch key {
			case "model", "route":
			case "stream":
				if _, ok := v.(bool); !ok {
					return nil, false
				}
			default:
				return nil, false
			}
		}
	} else {
		return nil, false
	}
	p := c.Pricing
	if p["currency"] != "USD" || p["unit"] != c.Unit || p["requires_quote"] != false {
		return nil, false
	}
	switch c.Unit {
	case "call", "token", "image", "second":
	default:
		return nil, false
	}
	filtered := map[string]any{"scope": "public_catalog", "currency": "USD", "unit": c.Unit, "requires_quote": false}
	for _, key := range []string{"amount_micro_usd", "amount", "prompt", "completion", "input_cache_read"} {
		if v, exists := p[key]; exists {
			s, ok := v.(string)
			if !ok || !candidateAmount.MatchString(s) {
				return nil, false
			}
			filtered[key] = s
		}
	}
	if c.Unit == "token" && (filtered["prompt"] == nil || filtered["completion"] == nil) {
		return nil, false
	}
	if c.Kind == "model" && c.Unit != "token" && filtered["amount"] == nil {
		return nil, false
	}
	if at, ok := p["as_of"].(string); ok && boundedText(at, 128) {
		filtered["as_of"] = at
	}
	c.Pricing = filtered
	checks := []string{}
	for _, s := range c.Checks {
		switch s {
		case "reviewed_operation", "current_contract", "request_covered", "provider_route_checked", "recheck_price_and_access":
			checks = append(checks, s)
		}
	}
	c.Checks = checks
	encoded, _ := json.Marshal(c)
	var out map[string]any
	d := json.NewDecoder(bytes.NewReader(encoded))
	d.UseNumber()
	_ = d.Decode(&out)
	return out, true
}
func candidateRecovery(obj map[string]any, enabled bool) (map[string]any, []any, bool) {
	if obj["type"] != "openany_error" {
		return nil, nil, false
	}
	code, _ := obj["code"].(string)
	if !candidateProviderCode(code) {
		return nil, nil, false
	}
	details, ok := obj["details"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	r, ok := details["recovery"].(map[string]any)
	if !ok || r["version"] != json.Number("1") || r["reason"] != "upstream_failure" {
		return nil, nil, false
	}
	state, _ := r["execution_state"].(string)
	charge, _ := r["charge_state"].(string)
	if !strings.Contains("|not_started|failed|unknown|started|completed|", "|"+state+"|") || state == "" || !strings.Contains("|not_charged|charged|unknown|pending|", "|"+charge+"|") || charge == "" {
		return nil, nil, false
	}
	core := map[string]any{"version": json.Number("1"), "reason": "upstream_failure", "execution_state": state, "charge_state": charge, "primary_action": "check_original_request", "next_step_target": "https://openany.si/dashboard?page=usage"}
	for _, key := range []string{"recovery_id", "alternatives_status", "replay_safety"} {
		if s, ok := r[key].(string); ok && publicToken.MatchString(s) {
			core[key] = s
		}
	}
	if !enabled || details["alternatives_version"] != json.Number("1") || r["primary_action"] != "inspect_alternatives" || r["replay_safety"] != "safe" || r["alternatives_status"] != "available" || (state != "not_started" && state != "failed") || charge != "not_charged" {
		return core, nil, true
	}
	if cost, ok := obj["openany_cost"]; ok && cost != json.Number("0") {
		return core, nil, true
	}
	values, ok := details["alternatives"].([]any)
	if !ok || len(values) < 1 || len(values) > 3 {
		return core, nil, true
	}
	encoded, _ := json.Marshal(values)
	if len(encoded) > 16384 {
		return core, nil, true
	}
	out := []any{}
	for _, v := range values {
		c, ok := boundedCandidate(v)
		if !ok {
			return core, nil, true
		}
		out = append(out, c)
	}
	if r["next_step_target"] != out[0].(map[string]any)["definition_path"] {
		return core, nil, true
	}
	core["primary_action"] = "inspect_alternatives"
	core["next_step_target"] = r["next_step_target"]
	return core, out, true
}
