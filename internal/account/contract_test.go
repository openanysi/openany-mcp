package account

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestOnboardingSharedContractPolicyCanBeConsumed(t *testing.T) {
	b, err := os.ReadFile("testdata/onboarding-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples map[string]json.RawMessage
	if err = json.Unmarshal(b, &examples); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"publicEnabled", "publicDisabled"} {
		t.Run(key, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(examples[key])
			}))
			defer s.Close()
			c, err := NewClient(s.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			p, err := c.PublicPolicy(context.Background(), "en")
			want := int64(0)
			if key == "publicEnabled" {
				want = 100000
			}
			if err != nil || p.TrialAmount() != want || p.ReferralReward.Calculation != "marginal_tiers" || p.ReferralReward.Recipient != "inviter" || len(p.ReferralReward.Tiers) != 4 || p.Links.PublicPolicy != PublicURL {
				t.Fatalf("backend policy contract changed: %#v %v", p, err)
			}
		})
	}
}
