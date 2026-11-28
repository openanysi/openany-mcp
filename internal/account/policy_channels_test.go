package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestPolicyTrialAmountOnlyWhenUniformAcrossEligibleChannels(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		want   int64
	}{
		{"legacy_equal", nil, 100000},
		{"explicit_equal", func(c map[string]any) {
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "100000", "cli_email": "100000"}
		}, 100000},
		{"different_amounts", func(c map[string]any) {
			c["amountMicroUsd"] = "250000"
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "250000", "cli_email": "100000"}
		}, 0},
		{"wallet_disabled", func(c map[string]any) { c["eligibleChannels"] = []string{"cli_email"} }, 100000},
		{"disabled", func(c map[string]any) { c["status"] = "disabled" }, 0},
		{"manual", func(c map[string]any) { c["automatic"] = false }, 0},
		{"wrong_currency", func(c map[string]any) { c["currency"] = "CNY" }, 0},
		{"no_channels", func(c map[string]any) { c["eligibleChannels"] = []string{} }, 0},
		{"duplicate_channel", func(c map[string]any) { c["eligibleChannels"] = []string{"cli_email", "cli_email"} }, 0},
		{"blank_channel", func(c map[string]any) { c["eligibleChannels"] = []string{""} }, 0},
		{"empty_map", func(c map[string]any) { c["amountMicroUsdByChannel"] = map[string]string{} }, 0},
		{"missing_channel", func(c map[string]any) { c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "100000"} }, 0},
		{"ineligible_channel", func(c map[string]any) {
			c["eligibleChannels"] = []string{"cli_email"}
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "250000", "cli_email": "100000"}
		}, 0},
		{"inconsistent_primary", func(c map[string]any) {
			c["amountMicroUsdByChannel"] = map[string]string{"agent_wallet": "250000", "cli_email": "100000"}
		}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credit := map[string]any{"automatic": true, "status": "enabled", "currency": "USD", "amountMicroUsd": "100000", "eligibleChannels": []string{"agent_wallet", "cli_email"}}
			if tc.change != nil {
				tc.change(credit)
			}
			p := decodeChannelPolicy(t, credit)
			if got := p.TrialAmount(); got != tc.want {
				t.Fatalf("uniform credit = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestPolicyRejectsMalformedPerChannelAmounts(t *testing.T) {
	for _, raw := range []string{"", "0", "-1", "+100000", "0100000", " 100000", "100000 ", "0.10", "1e5", "9223372036854775808"} {
		t.Run("amount="+raw, func(t *testing.T) {
			p := decodeChannelPolicy(t, map[string]any{"automatic": true, "status": "enabled", "currency": "USD", "amountMicroUsd": "100000", "eligibleChannels": []string{"agent_wallet", "cli_email"},
				"amountMicroUsdByChannel": map[string]string{"agent_wallet": "100000", "cli_email": raw}})
			if p.TrialAmount() != 0 {
				t.Fatal("malformed channel amount was used as a universal offer")
			}
		})
	}
}

func decodeChannelPolicy(t *testing.T, credit map[string]any) *Policy {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"version": 1, "status": "available", "policyRevision": "channel-test", "signupCredit": credit})
	if err != nil {
		t.Fatal(err)
	}
	var p Policy
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func TestChannelPolicySurvivesPublicAndAuthenticatedClients(t *testing.T) {
	raw, err := os.ReadFile("testdata/onboarding-channel-amounts.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]struct{ Data map[string]any }
	if err := json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"equal", "mixed", "wallet_disabled", "disabled"} {
		t.Run(name, func(t *testing.T) {
			policy := fixtures[name].Data
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var data any = policy
				switch r.URL.Path {
				case policyPath:
					if r.Header.Get("Authorization") != "" {
						t.Error("public policy received credentials")
					}
				case accountPath:
					if r.Header.Get("Authorization") != "Bearer local-test-key" {
						t.Error("missing local fixture credential")
					}
					data = map[string]any{"version": 1, "authentication": "authenticated", "businessRequestStarted": false, "publicBenefits": policy,
						"balance": map[string]string{"status": "available", "availableMicroUsd": "70000"}, "signupCredit": map[string]string{"status": "credited", "grantedAmountMicroUsd": "100000"}, "referral": map[string]string{"status": "restricted"}}
				default:
					t.Error("unexpected business request", r.URL.Path)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer server.Close()
			client, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			public, err := client.PublicPolicy(t.Context(), "en")
			if err != nil {
				t.Fatal(err)
			}
			summary, err := client.Summary(t.Context(), "en", http.Header{"Authorization": {"Bearer local-test-key"}})
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []*Policy{public, summary.PublicBenefits} {
				encoded, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				var got map[string]any
				if err = json.Unmarshal(encoded, &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got["signupCredit"], policy["signupCredit"]) {
					t.Fatalf("channel rules lost in round trip: got %v, want %v", got["signupCredit"], policy["signupCredit"])
				}
			}
			if summary.Balance.AvailableMicroUSD != "70000" || summary.SignupCredit.GrantedAmountMicroUSD != "100000" || summary.Referral.Status != "restricted" {
				t.Fatal("public offer changed actual balance, credited grant or referral access")
			}
		})
	}
}
