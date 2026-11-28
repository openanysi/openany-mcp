package gateway

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// The v2 wire whitelist is independent of the legacy parser. In particular,
// absent execution fields cannot be synthesized for a read-only candidate.
type candidateCompatibilityV2 struct {
	Level           string                `json:"level"`
	Differences     []candidateDifference `json:"differences"`
	InputMapping    []candidateMapping    `json:"input_mapping"`
	Basis           string                `json:"basis"`
	MissingEvidence []string              `json:"missing_evidence"`
}
type publicCandidateV2 struct {
	CandidateID   string                   `json:"candidate_id"`
	Kind          string                   `json:"kind"`
	Relation      string                   `json:"relation"`
	Operation     string                   `json:"operation_key,omitempty"`
	Provider      string                   `json:"provider_id"`
	Name          string                   `json:"name"`
	APIID         string                   `json:"api_id,omitempty"`
	Revision      int                      `json:"api_revision,omitempty"`
	ModelID       string                   `json:"model_id,omitempty"`
	ModelRevision string                   `json:"model_revision,omitempty"`
	Method        string                   `json:"method,omitempty"`
	Path          string                   `json:"public_path,omitempty"`
	Definition    string                   `json:"definition_path"`
	Price         string                   `json:"price_micro_usd,omitempty"`
	Currency      string                   `json:"currency,omitempty"`
	Unit          string                   `json:"billing_unit,omitempty"`
	Compatibility candidateCompatibilityV2 `json:"compatibility"`
	Execution     *candidateExecution      `json:"execution,omitempty"`
	Pricing       map[string]any           `json:"pricing"`
	Availability  map[string]any           `json:"availability"`
	Checks        []string                 `json:"checks"`
}

func strictCandidateDecode(raw any, target any, limit int) bool {
	b, err := json.Marshal(raw)
	if err != nil || len(b) > limit {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	d.DisallowUnknownFields()
	return d.Decode(target) == nil
}

func boundedAlternativesMeta(raw any) (map[string]any, bool) {
	rawMap, ok := raw.(map[string]any)
	if !ok || !allowedMapKeys(rawMap, "rule_version", "snapshot_revision", "generated_at") {
		return nil, false
	}
	var m struct {
		Rule      string `json:"rule_version"`
		Snapshot  string `json:"snapshot_revision"`
		Generated string `json:"generated_at"`
	}
	if !strictCandidateDecode(raw, &m, 512) || !publicToken.MatchString(m.Rule) || !candidateHash.MatchString(m.Snapshot) || len(m.Generated) > 64 {
		return nil, false
	}
	if _, err := time.Parse(time.RFC3339, m.Generated); err != nil {
		return nil, false
	}
	return map[string]any{"rule_version": m.Rule, "snapshot_revision": m.Snapshot, "generated_at": m.Generated}, true
}

var evidenceCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var fieldName = regexp.MustCompile(`^[A-Za-z0-9_./~\[\]-]{1,240}$`)

func mappingReferenceV2(s string) bool {
	location, name, ok := strings.Cut(s, ":")
	if !ok || !boundedText(s, 256) || !fieldName.MatchString(name) || (location != "path" && location != "query" && location != "body") {
		return false
	}
	// Exact business token names are safe only outside authentication containers.
	for _, part := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return r == '.' || r == '/' || r == '[' || r == ']' }) {
		if part == "token_address" || part == "token_symbol" {
			continue
		}
		normalized := strings.ReplaceAll(strings.ReplaceAll(part, "_", ""), "-", "")
		for _, auth := range []string{"authorization", "authentication", "credential", "apikey", "token", "password", "secret", "bearer", "cookie", "session", "privatekey", "signingkey", "accesskey"} {
			if strings.Contains(normalized, auth) {
				return false
			}
		}
		if part == "auth" || part == "oauth" {
			return false
		}
	}
	return true
}

func allowedMapKeys(m map[string]any, keys ...string) bool {
	for key := range m {
		found := false
		for _, allowed := range keys {
			if key == allowed {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return m != nil
}

func boundedCandidateV2(raw any) (map[string]any, bool) {
	var c publicCandidateV2
	if !strictCandidateDecode(raw, &c, 4096) {
		return nil, false
	}
	rawMap, ok := raw.(map[string]any)
	if !ok || !allowedMapKeys(rawMap, "candidate_id", "kind", "relation", "operation_key", "provider_id", "name", "api_id", "api_revision", "model_id", "model_revision", "method", "public_path", "definition_path", "price_micro_usd", "currency", "billing_unit", "compatibility", "execution", "pricing", "availability", "checks") || !candidateHash.MatchString(c.CandidateID) || !publicToken.MatchString(c.Provider) || c.Name == "" || !boundedText(c.Name, 128) {
		return nil, false
	}
	compatibilityMap, ok := rawMap["compatibility"].(map[string]any)
	if !ok || !allowedMapKeys(compatibilityMap, "level", "differences", "input_mapping", "basis", "missing_evidence") {
		return nil, false
	}
	for _, key := range []string{"differences", "input_mapping"} {
		rows, ok := compatibilityMap[key].([]any)
		if !ok {
			return nil, false
		}
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if !ok {
				return nil, false
			}
			if key == "differences" {
				if !allowedMapKeys(m, "code", "message") {
					return nil, false
				}
			} else if !allowedMapKeys(m, "from", "to") {
				return nil, false
			}
		}
	}
	if execution, present := rawMap["execution"]; present {
		if m, ok := execution.(map[string]any); !ok || !allowedMapKeys(m, "protocol", "method", "path", "definition_path", "request_overrides") {
			return nil, false
		}
	}
	k := c.Compatibility
	if k.Differences == nil || k.InputMapping == nil || k.MissingEvidence == nil || c.Checks == nil || (k.Basis != "contract_verified" && k.Basis != "catalog_inferred") || len(k.Differences) > 8 || len(k.InputMapping) > 32 || len(k.MissingEvidence) > 32 || len(c.Checks) > 32 {
		return nil, false
	}
	for _, d := range k.Differences {
		if !publicToken.MatchString(d.Code) || !boundedText(d.Message, 256) || strings.Contains(strings.ToLower(d.Message), "http") || strings.Contains(strings.ToLower(d.Message), "bearer") {
			return nil, false
		}
	}
	for _, m := range k.InputMapping {
		if !mappingReferenceV2(m.From) || !mappingReferenceV2(m.To) {
			return nil, false
		}
	}
	for _, code := range append(append([]string{}, k.MissingEvidence...), c.Checks...) {
		if !evidenceCode.MatchString(code) {
			return nil, false
		}
	}
	for _, code := range k.MissingEvidence {
		switch code {
		case "operation_scope", "output_contract", "read_only", "input_semantics", "input_schema", "pricing", "unsupported_constraint", "metadata_conflict", "request_contract", "current_contract":
		default:
			return nil, false
		}
	}
	checks := []string{}
	for _, code := range c.Checks {
		switch code {
		case "reviewed_operation", "operation_matched", "current_contract", "request_covered", "provider_route_checked", "recheck_price_and_access", "inspect_target_contract":
			checks = append(checks, code)
		}
	}
	c.Checks = checks
	if !allowedMapKeys(c.Availability, "published", "health", "basis") || c.Availability["published"] != true {
		return nil, false
	}
	if health := c.Availability["health"]; health != "unknown" && health != "observed_available" && health != "recent_failure" {
		return nil, false
	}
	if basis, exists := c.Availability["basis"]; exists {
		if basis != "current_catalog" && basis != "current_route_catalog" {
			return nil, false
		}
	}
	p := c.Pricing
	if !allowedMapKeys(p, "status", "scope", "currency", "unit", "requires_quote", "as_of", "amount_micro_usd", "amount", "prompt", "completion", "input_cache_read") || p["scope"] != "public_catalog" {
		return nil, false
	}
	for _, key := range []string{"amount_micro_usd", "amount", "prompt", "completion", "input_cache_read"} {
		if value, exists := p[key]; exists {
			amount, ok := value.(string)
			if !ok || !candidateAmount.MatchString(amount) {
				return nil, false
			}
		}
	}
	if value, exists := p["as_of"]; exists {
		at, ok := value.(string)
		if !ok || !boundedText(at, 128) {
			return nil, false
		}
	}
	if c.Kind == "api" {
		for _, key := range []string{"model_id", "model_revision"} {
			if _, present := rawMap[key]; present {
				return nil, false
			}
		}
	} else if c.Kind == "model" {
		for _, key := range []string{"api_id", "api_revision"} {
			if _, present := rawMap[key]; present {
				return nil, false
			}
		}
	}
	needs := k.Level == "needs_inspection"
	if needs {
		if len(k.InputMapping) != 0 {
			return nil, false
		}
		if c.Kind == "model" {
			if (c.Relation != "same_model" && c.Relation != "similar_model") || c.ModelID == "" || !boundedText(c.ModelID, 128) || !candidateHash.MatchString(c.ModelRevision) || !definitionPath(c.Definition, "model", c.ModelID) || c.Operation != "chat.completion" || k.Basis != "catalog_inferred" || len(k.MissingEvidence) == 0 {
				return nil, false
			}
			if c.Relation == "similar_model" {
				changed := false
				for _, d := range k.Differences {
					changed = changed || d.Code == "model_changed"
				}
				if !changed {
					return nil, false
				}
			}
		} else if c.Kind != "api" || c.Relation != "related_operation" || !candidateUUID.MatchString(c.APIID) || c.Revision < 1 || !definitionPath(c.Definition, "api", c.APIID) || c.ModelID != "" || c.ModelRevision != "" {
			return nil, false
		}
		for _, key := range []string{"execution", "method", "public_path"} {
			if _, present := rawMap[key]; present {
				return nil, false
			}
		}
		if _, present := rawMap["operation_key"]; present && !publicToken.MatchString(c.Operation) {
			return nil, false
		}
		if p["status"] != "known" {
			if (p["status"] != "unknown" && p["status"] != "requires_quote") || p["requires_quote"] != true {
				return nil, false
			}
			for _, key := range []string{"amount_micro_usd", "amount", "prompt", "completion", "input_cache_read"} {
				if _, present := p[key]; present {
					return nil, false
				}
			}
			for _, key := range []string{"price_micro_usd", "currency", "billing_unit"} {
				if _, present := rawMap[key]; present {
					return nil, false
				}
			}
			if _, present := p["currency"]; present {
				return nil, false
			}
			if _, present := p["unit"]; present {
				return nil, false
			}
			return candidateV2Map(c), true
		}
		if c.Kind == "model" {
			if c.Currency != "USD" || p["currency"] != "USD" || p["unit"] != c.Unit || p["requires_quote"] != false || c.Price != "" {
				return nil, false
			}
			if c.Unit == "token" {
				if p["prompt"] == nil || p["completion"] == nil {
					return nil, false
				}
			} else if c.Unit != "call" || p["amount"] == nil {
				return nil, false
			}
			return candidateV2Map(c), true
		}
		// A known inspection price still has to be a real fixed API price.
		if c.Currency != "USD" || c.Unit != "call" || !candidateAmount.MatchString(c.Price) || p["amount_micro_usd"] != c.Price || p["currency"] != "USD" || p["unit"] != "call" || p["requires_quote"] != false {
			return nil, false
		}
		return candidateV2Map(c), true
	}
	// Any candidate with an execution entry needs complete contract evidence.
	// Incomplete high grades are rejected, never repaired or promoted here.
	if k.Basis != "contract_verified" || len(k.MissingEvidence) != 0 {
		return nil, false
	}
	if (c.Kind == "api" && k.Level != "direct" && k.Level != "requires_adaptation") || (c.Kind == "model" && k.Level != "direct" && k.Level != "requires_adaptation" && k.Level != "similar") || c.Execution == nil || p["status"] != "known" {
		return nil, false
	}
	if c.Kind == "model" && ((c.Relation == "same_model" && k.Level == "similar") || (c.Relation == "similar_model" && k.Level != "similar")) {
		return nil, false
	}
	if c.Kind == "api" && c.Unit != "call" {
		return nil, false
	}
	// Reuse the established execution and known-price validation after removing
	// v2-only evidence; validated business mappings are retained in the v2 output.
	legacy := publicCandidate{CandidateID: c.CandidateID, Kind: c.Kind, Relation: c.Relation, Operation: c.Operation, Provider: c.Provider, Name: c.Name, APIID: c.APIID, Revision: c.Revision, ModelID: c.ModelID, ModelRevision: c.ModelRevision, Method: c.Method, Path: c.Path, Definition: c.Definition, Price: c.Price, Currency: c.Currency, Unit: c.Unit, Compatibility: candidateCompatibility{Level: k.Level, Differences: k.Differences, InputMapping: []candidateMapping{}}, Execution: *c.Execution, Pricing: p, Availability: c.Availability, Checks: c.Checks}
	if _, ok := boundedCandidate(legacy); !ok {
		return nil, false
	}
	return candidateV2Map(c), true
}

func candidateV2Map(c publicCandidateV2) map[string]any {
	b, _ := json.Marshal(c)
	var out map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	_ = d.Decode(&out)
	return out
}

func candidateRecoveryV2(obj map[string]any, enabled bool) (map[string]any, []any, bool) {
	core, _, ok := candidateRecoveryV1(obj, false)
	if !ok {
		return nil, nil, false
	}
	core["alternatives_status"] = "none"
	d := obj["details"].(map[string]any)
	r := d["recovery"].(map[string]any)
	state, charge := r["execution_state"], r["charge_state"]
	if status := r["alternatives_status"]; status == "unavailable" || status == "skipped" {
		core["alternatives_status"] = status
	}
	if !enabled || r["alternatives_status"] != "available" || (r["primary_action"] != "inspect_alternatives" && r["primary_action"] != "inspect_related_apis") || (state != "not_started" && state != "failed") || charge != "not_charged" {
		return core, nil, true
	}
	if cost, present := obj["openany_cost"]; present && cost != json.Number("0") {
		return core, nil, true
	}
	meta, valid := boundedAlternativesMeta(d["alternatives_meta"])
	if !valid {
		return core, nil, true
	}
	target, _ := r["next_step_target"].(string)
	if !officialDefinitionTarget(target) {
		return core, nil, true
	}
	values, valid := d["alternatives"].([]any)
	if !valid || len(values) < 1 || len(values) > 3 {
		return core, nil, true
	}
	b, err := json.Marshal(map[string]any{"alternatives": values, "alternatives_meta": meta})
	if err != nil || len(b) > 16384 {
		return core, nil, true
	}
	out := []any{}
	highTarget := ""
	safety := r["replay_safety"]
	readOnly := r["primary_action"] == "inspect_related_apis" || safety == "unknown"
	for _, raw := range values {
		c, valid := boundedCandidateV2(raw)
		if !valid {
			continue
		}
		if readOnly && c["compatibility"].(map[string]any)["level"] != "needs_inspection" {
			continue
		}
		out = append(out, c)
		if c["compatibility"].(map[string]any)["level"] != "needs_inspection" && highTarget == "" {
			highTarget = c["definition_path"].(string)
		}
	}
	if len(out) == 0 {
		return core, nil, true
	}
	if safety != "safe" && !(safety == "unknown" && highTarget == "") {
		return core, nil, true
	}
	if highTarget != "" {
		if r["primary_action"] != "inspect_alternatives" {
			return core, nil, true
		}
		core["primary_action"], core["next_step_target"] = "inspect_alternatives", highTarget
	} else if out[0].(map[string]any)["kind"] == "model" {
		core["primary_action"], core["next_step_target"] = "inspect_alternatives", out[0].(map[string]any)["definition_path"]
	} else {
		core["primary_action"], core["next_step_target"] = "inspect_related_apis", out[0].(map[string]any)["definition_path"]
	}
	core["alternatives_status"] = "available"
	core["reason"] = r["reason"]
	return core, out, true
}
