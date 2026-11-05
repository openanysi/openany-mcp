package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicRulePricingPreservesNullAndExcludesPrivateFields(t *testing.T) {
	var raw map[string]any
	json.Unmarshal([]byte(`{"mode":"rules","currency":"USD","supplier":"private-marker","policy":{"policy_version":1,"supplier":{"secret":"private-marker"},"parameters":[{"id":"format","location":"query","name":"format","type":"string","required":false,"default":null,"internal":"private-marker"}],"sale":{"base_price_micro_usd":"100","rounding":"ceil","max_cost_micro_usd":"500","variants":[{"id":"nil","label":"Null format","price_micro_usd":"200","when":[{"parameter":"format","op":"eq","value":null,"secret":"private-marker"}],"supplier":"private-marker"}],"addons":[],"quantities":[]}}}`), &raw)
	p := publicPricing(raw)
	encoded, _ := json.Marshal(p)
	if strings.Contains(string(encoded), "private-marker") || p["status"] != "available" {
		t.Fatal("private pricing leaked or public pricing unavailable", string(encoded))
	}
	policy := p["policy"].(map[string]any)
	param := policy["parameters"].([]any)[0].(map[string]any)
	if value, exists := param["default"]; !exists || value != nil {
		t.Fatal("explicit null default lost")
	}
	when := policy["sale"].(map[string]any)["variants"].([]any)[0].(map[string]any)["when"].([]any)[0].(map[string]any)
	if value, exists := when["value"]; !exists || value != nil {
		t.Fatal("null price condition lost")
	}
}
