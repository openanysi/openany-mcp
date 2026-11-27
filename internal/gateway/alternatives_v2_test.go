package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/openany/openany-mcp/internal/model"
)

func candidateV2Fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("testdata/recovery-candidates/v2/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	d := json.NewDecoder(strings.NewReader(string(b)))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func v2Parts(v map[string]any) (map[string]any, map[string]any, map[string]any) {
	d := v["details"].(map[string]any)
	return d, d["recovery"].(map[string]any), d["alternatives"].([]any)[0].(map[string]any)
}

func TestD31V2SharedCandidatesPreserveGradesAndMetadata(t *testing.T) {
	for _, name := range []string{"direct", "requires_adaptation", "needs_inspection", "mixed", "model"} {
		t.Run(name, func(t *testing.T) {
			v := candidateV2Fixture(t, name)
			d, recovery, _ := v2Parts(v)
			e := publicFailure(v, 503, "original", true).Value["error"].(map[string]any)
			if e["alternatives_version"] != json.Number("2") || e["execution_state"] != "failed" || e["charge_state"] != "not_charged" {
				t.Fatalf("lost v2 facts: %#v", e)
			}
			if !reflect.DeepEqual(e["alternatives_meta"], d["alternatives_meta"]) || e["recovery"].(map[string]any)["primary_action"] != recovery["primary_action"] {
				t.Fatalf("metadata/action changed: %#v", e)
			}
			if !reflect.DeepEqual(e["alternatives"], d["alternatives"]) {
				t.Fatalf("candidate fields changed: %#v", e["alternatives"])
			}
		})
	}
}

func TestD34V2FilteringRecomputesActionAndTarget(t *testing.T) {
	v := candidateV2Fixture(t, "mixed")
	d, _, first := v2Parts(v)
	first["execution"].(map[string]any)["path"] = "https://evil.example/run"
	e := publicFailure(v, 503, "original", true).Value["error"].(map[string]any)
	a, ok := e["alternatives"].([]any)
	if !ok || len(a) != 1 || e["recovery"].(map[string]any)["primary_action"] != "inspect_related_apis" || e["recovery"].(map[string]any)["next_step_target"] != a[0].(map[string]any)["definition_path"] {
		t.Fatalf("did not downgrade: %#v", e)
	}
	d["alternatives"].([]any)[1].(map[string]any)["execution"] = map[string]any{}
	e = publicFailure(v, 503, "original", true).Value["error"].(map[string]any)
	r := e["recovery"].(map[string]any)
	if e["alternatives"] != nil || e["alternatives_meta"] != nil || r["primary_action"] != "check_original_request" || r["next_step_target"] != "https://openany.si/dashboard?page=usage" || r["alternatives_status"] == "available" {
		t.Fatalf("dangling action: %#v", e)
	}
}

func TestD31V2UnknownReplayOnlyAllowsReadOnlyKnownFailure(t *testing.T) {
	for _, tc := range []struct {
		name, state, charge, safety string
		want                        bool
	}{
		{"needs_inspection", "failed", "not_charged", "unknown", true},
		{"needs_inspection", "not_started", "not_charged", "unknown", true},
		{"direct", "failed", "not_charged", "unknown", false},
		{"mixed", "failed", "not_charged", "unknown", true},
		{"needs_inspection", "unknown", "not_charged", "unknown", false},
		{"needs_inspection", "failed", "unknown", "safe", false},
		{"needs_inspection", "failed", "not_charged", "unsafe", false},
	} {
		t.Run(strings.Join([]string{tc.name, tc.state, tc.charge, tc.safety}, "/"), func(t *testing.T) {
			v := candidateV2Fixture(t, tc.name)
			_, r, _ := v2Parts(v)
			r["execution_state"], r["charge_state"], r["replay_safety"] = tc.state, tc.charge, tc.safety
			e := publicFailure(v, 503, "original", true).Value["error"].(map[string]any)
			if (e["alternatives"] != nil) != tc.want || e["execution_state"] != tc.state || e["charge_state"] != tc.charge {
				t.Fatalf("unsafe/lost state: %#v", e)
			}
			if tc.want && tc.safety == "unknown" {
				for _, raw := range e["alternatives"].([]any) {
					c := raw.(map[string]any)
					if c["compatibility"].(map[string]any)["level"] != "needs_inspection" || c["execution"] != nil {
						t.Fatal("unknown replay advertised executable candidate")
					}
				}
			}
		})
	}
}

func TestD34V2PlatformReadOnlyActionNeverUpgradesMixedCandidates(t *testing.T) {
	v := candidateV2Fixture(t, "mixed")
	_, r, _ := v2Parts(v)
	r["primary_action"] = "inspect_related_apis"
	e := publicFailure(v, 503, "original", true).Value["error"].(map[string]any)
	a, ok := e["alternatives"].([]any)
	if !ok || len(a) != 1 {
		t.Fatalf("read-only candidate lost: %#v", e)
	}
	c := a[0].(map[string]any)
	if c["compatibility"].(map[string]any)["level"] != "needs_inspection" || c["execution"] != nil || e["recovery"].(map[string]any)["primary_action"] != "inspect_related_apis" || e["recovery"].(map[string]any)["next_step_target"] != c["definition_path"] {
		t.Fatalf("platform read-only action upgraded: %#v", e)
	}
}

func TestD33D35V2RejectsInvalidFieldsAndKeepsCore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any, map[string]any, map[string]any)
	}{
		{"unknown_version", func(d, r, c map[string]any) { d["alternatives_version"] = json.Number("3") }},
		{"string_version", func(d, r, c map[string]any) { d["alternatives_version"] = "2" }},
		{"bad_meta", func(d, r, c map[string]any) {
			d["alternatives_meta"].(map[string]any)["snapshot_revision"] = "unverified"
		}},
		{"bad_timestamp", func(d, r, c map[string]any) { d["alternatives_meta"].(map[string]any)["generated_at"] = "tomorrow" }},
		{"extra_meta", func(d, r, c map[string]any) { d["alternatives_meta"].(map[string]any)["private"] = "secret" }},
		{"extra_candidate", func(d, r, c map[string]any) { c["private"] = "secret" }},
		{"bad_relation", func(d, r, c map[string]any) { c["relation"] = "same_operation" }},
		{"execution_null", func(d, r, c map[string]any) { c["execution"] = nil }},
		{"method", func(d, r, c map[string]any) { c["method"] = "GET" }},
		{"public_path", func(d, r, c map[string]any) { c["public_path"] = "/v1/agent-apis/target/read" }},
		{"input_mapping", func(d, r, c map[string]any) {
			c["compatibility"].(map[string]any)["input_mapping"] = []any{map[string]any{"from": "query:q", "to": "query:q"}}
		}},
		{"unknown_price_zero", func(d, r, c map[string]any) { c["pricing"].(map[string]any)["amount_micro_usd"] = "0" }},
		{"unknown_top_price", func(d, r, c map[string]any) { c["price_micro_usd"] = "0" }},
		{"missing_evidence_type", func(d, r, c map[string]any) {
			c["compatibility"].(map[string]any)["missing_evidence"] = []any{map[string]any{"code": "output_contract"}}
		}},
		{"oversize_item", func(d, r, c map[string]any) { c["name"] = strings.Repeat("x", 4097) }},
		{"too_many", func(d, r, c map[string]any) { d["alternatives"] = []any{c, c, c, c} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := candidateV2Fixture(t, "needs_inspection")
			d, r, c := v2Parts(v)
			tc.change(d, r, c)
			result := publicFailure(v, 503, "original", true)
			e := result.Value["error"].(map[string]any)
			if e["alternatives"] != nil || e["execution_state"] != "failed" || e["charge_state"] != "not_charged" || result.Value["request_id"] != "original" || e["code"] != v["code"] {
				t.Fatalf("accepted or lost core: %#v", result)
			}
			if e["recovery"].(map[string]any)["primary_action"] != "check_original_request" {
				t.Fatalf("dangling action: %#v", e)
			}
		})
	}
}

func TestD35V2MappingDistinguishesBusinessTokensFromCredentials(t *testing.T) {
	for _, name := range []string{"token_address", "token_symbol", "authorization", "api_key", "api-key", "access_token", "refresh_token", "client_secret", "private_key", "signing_key", "access_key", "auth.token_address", "credentials.token_symbol"} {
		t.Run(name, func(t *testing.T) {
			v := candidateV2Fixture(t, "requires_adaptation")
			_, _, c := v2Parts(v)
			c["compatibility"].(map[string]any)["input_mapping"] = []any{map[string]any{"from": "query:" + name, "to": "body:" + name}}
			e := publicFailure(v, 503, "original", true).Value["error"].(map[string]any)
			want := name == "token_address" || name == "token_symbol"
			if (e["alternatives"] != nil) != want {
				t.Fatalf("mapping %s accepted=%v want=%v", name, e["alternatives"] != nil, want)
			}
		})
	}
}

func TestD32V1RemainsStrictAndV2DisabledKeepsKnownState(t *testing.T) {
	v := candidateFixture(t)
	d := v["details"].(map[string]any)
	d["alternatives"].([]any)[0].(map[string]any)["compatibility"].(map[string]any)["basis"] = "contract_verified"
	if publicFailure(v, 503, "old", true).Value["error"].(map[string]any)["alternatives"] != nil {
		t.Fatal("v1 accepted v2 fields")
	}
	v = candidateV2Fixture(t, "direct")
	legacy, alternatives, ok := candidateRecoveryV1(v, true)
	if !ok || alternatives != nil || legacy["execution_state"] != "failed" || legacy["charge_state"] != "not_charged" || legacy["primary_action"] != "check_original_request" {
		t.Fatalf("frozen legacy parser upgraded v2: %#v / %#v", legacy, alternatives)
	}
	e := publicFailure(v, 503, "new", false).Value["error"].(map[string]any)
	if e["alternatives"] != nil || e["execution_state"] != "failed" || e["charge_state"] != "not_charged" {
		t.Fatalf("disabled lost facts: %#v", e)
	}
}

func TestD35V2StrictNestedWhitelistAndBoundedMeta(t *testing.T) {
	for _, change := range []func(map[string]any, map[string]any){
		func(d, c map[string]any) { c["compatibility"].(map[string]any)["Level"] = "needs_inspection" },
		func(d, c map[string]any) {
			c["pricing"].(map[string]any)["as_of"] = map[string]any{"secret": "private"}
		},
		func(d, c map[string]any) { c["pricing"].(map[string]any)["as_of"] = strings.Repeat("x", 129) },
		func(d, c map[string]any) {
			d["alternatives_meta"].(map[string]any)["rule_version"] = strings.Repeat("x", 129)
		},
		func(d, c map[string]any) { d["alternatives_meta"].(map[string]any)["Rule_version"] = "api-v2" },
		func(d, c map[string]any) { c["compatibility"].(map[string]any)["basis"] = "similarity_guessed" },
	} {
		v := candidateV2Fixture(t, "needs_inspection")
		d, _, c := v2Parts(v)
		change(d, c)
		if e := publicFailure(v, 503, "original", true).Value["error"].(map[string]any); e["alternatives"] != nil {
			t.Fatalf("invalid nested data accepted: %#v", e)
		}
	}
}

func TestD31V2ResultLimitRetainsCoreChargeAndConservativeRecovery(t *testing.T) {
	v := candidateV2Fixture(t, "needs_inspection")
	r := publicFailure(v, 503, "limited", true)
	r.Value["presentation"] = strings.Repeat("x", 20000)
	c, _ := New("https://openany.si", 1, 1, 4096, 8192)
	defer c.Close()
	r = c.LimitResult(r)
	e := r.Value["error"].(map[string]any)
	if e["execution_state"] != "failed" || e["charge_state"] != "not_charged" || e["original_code"] != v["code"] || e["alternatives"] != nil || r.Value["request_id"] != "limited" || r.Value["openany_cost"] != json.Number("0") {
		t.Fatalf("limit discarded core facts: %#v", r.Value)
	}
}

func TestD35V2OnlyOuterPlatformFailureProjectsCandidates(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		outer     bool
		wantError bool
	}{
		{"platform_error", 503, true, true}, {"success_spoof", 200, true, false}, {"nested_supplier_error", 503, false, true}, {"success_nested_error", 200, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := candidateV2Fixture(t, "direct")
			if !tc.outer {
				v = map[string]any{"error": v}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(v)
			}))
			defer server.Close()
			client, err := New(server.URL, time.Second, time.Second, 32768, 131072)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.EnableAlternatives(true)
			r := client.Call(context.Background(), model.Request{Method: "GET", PublicPath: "/test"}, nil, nil)
			if r.IsError != tc.wantError {
				t.Fatalf("wrong failure classification: %#v", r)
			}
			if e, ok := r.Value["error"].(map[string]any); ok {
				if (e["alternatives"] != nil) != (tc.outer && tc.status >= 400) {
					t.Fatalf("supplier instructions trusted: %#v", e)
				}
			} else if r.Value["data"] == nil {
				t.Fatal("success business data lost")
			}
		})
	}
}

func TestD31V2InspectionPriceStatesAndControlledEvidence(t *testing.T) {
	for _, status := range []string{"known", "requires_quote", "unknown"} {
		t.Run(status, func(t *testing.T) {
			v := candidateV2Fixture(t, "needs_inspection")
			_, _, c := v2Parts(v)
			if status == "known" {
				c["price_micro_usd"], c["currency"], c["billing_unit"] = "1200", "USD", "call"
				c["pricing"] = map[string]any{"status": "known", "scope": "public_catalog", "requires_quote": false, "amount_micro_usd": "1200", "currency": "USD", "unit": "call"}
			} else {
				c["pricing"].(map[string]any)["status"] = status
			}
			c["checks"] = []any{"operation_matched", "unknown_future_check"}
			e := publicFailure(v, 503, "original", true).Value["error"].(map[string]any)
			a, ok := e["alternatives"].([]any)
			if !ok {
				t.Fatalf("valid %s inspection price rejected: %#v", status, e)
			}
			p := a[0].(map[string]any)["pricing"].(map[string]any)
			if p["status"] != status || (status != "known" && (p["amount_micro_usd"] != nil || p["currency"] != nil || p["unit"] != nil)) {
				t.Fatalf("price invented: %#v", p)
			}
			if checks := a[0].(map[string]any)["checks"].([]any); len(checks) != 1 || checks[0] != "operation_matched" {
				t.Fatalf("uncontrolled checks: %#v", checks)
			}
			c["compatibility"].(map[string]any)["missing_evidence"] = []any{"invented_code"}
			if publicFailure(v, 503, "original", true).Value["error"].(map[string]any)["alternatives"] != nil {
				t.Fatal("unknown missing evidence code accepted")
			}
		})
	}
}
