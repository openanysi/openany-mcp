package account

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	policyRefresh = 30 * time.Second
	policyMaxAge  = 120 * time.Second
	PublicURL     = "https://openany.si" + policyPath
)

type Links struct {
	Account       string `json:"account,omitempty"`
	Register      string `json:"register,omitempty"`
	TopUp         string `json:"topUp,omitempty"`
	Setup         string `json:"setup,omitempty"`
	ReferralRules string `json:"referralRules,omitempty"`
	PublicPolicy  string `json:"publicPolicy,omitempty"`
}

type Policy struct {
	Version        int            `json:"version"`
	PolicyRevision string         `json:"policyRevision"`
	Status         string         `json:"status"`
	AsOf           string         `json:"asOf,omitempty"`
	Lang           string         `json:"lang,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	Links          Links          `json:"links"`
	SignupCredit   SignupCredit   `json:"signupCredit"`
	ReferralReward ReferralReward `json:"referralReward"`
}

type SignupCredit struct {
	Automatic        bool     `json:"automatic"`
	Status           string   `json:"status"`
	AmountMicroUSD   string   `json:"amountMicroUsd"`
	Currency         string   `json:"currency"`
	EligibleChannels []string `json:"eligibleChannels"`
	RulesURL         string   `json:"rulesUrl"`
}

type ReferralReward struct {
	Automatic            bool   `json:"automatic"`
	Recipient            string `json:"recipient"`
	RulesURL             string `json:"rulesUrl"`
	Tiers                []Tier `json:"tiers"`
	MinimumTopupMicroUSD string `json:"minimumTopupMicroUsd"`
	MinimumIsExclusive   bool   `json:"minimumIsExclusive"`
	CapMicroUSD          string `json:"capMicroUsd"`
	CapAppliesTo         string `json:"capAppliesTo"`
	Calculation          string `json:"calculation"`
}

type Tier struct {
	FromMicroUSD        string `json:"fromMicroUsd"`
	ToMicroUSD          string `json:"toMicroUsd"`
	DisplayFromMicroUSD string `json:"displayFromMicroUsd"`
	DisplayToMicroUSD   string `json:"displayToMicroUsd"`
	RateBPS             int    `json:"rateBps"`
}

func (p *Policy) valid() bool {
	return p != nil && p.Version == 1 && p.PolicyRevision != "" && p.Status == "available"
}

func (p *Policy) TrialAmount() int64 {
	if !p.valid() || p.SignupCredit.Status != "enabled" || !p.SignupCredit.Automatic ||
		p.SignupCredit.Currency != "USD" || len(p.SignupCredit.EligibleChannels) == 0 {
		return 0
	}
	amount, err := strconv.ParseInt(p.SignupCredit.AmountMicroUSD, 10, 64)
	if err != nil || amount <= 0 || strconv.FormatInt(amount, 10) != p.SignupCredit.AmountMicroUSD {
		return 0
	}
	return amount
}

type policyEntry struct {
	policy   *Policy
	loadedAt time.Time
}

// PolicyCache publishes immutable public snapshots. The caller owns the
// lifetime of Run and cancels its context before closing the account client.
type PolicyCache struct {
	client *Client
	mu     sync.RWMutex
	byLang map[string]policyEntry
}

func NewPolicyCache(client *Client) *PolicyCache {
	return &PolicyCache{client: client, byLang: make(map[string]policyEntry, len(languages))}
}

func (c *PolicyCache) Current(lang string) *Policy {
	return c.current(lang, time.Now())
}

func (c *PolicyCache) current(lang string, now time.Time) *Policy {
	if c == nil {
		return nil
	}
	if lang == "" {
		lang = "en"
	}
	c.mu.RLock()
	entry := c.byLang[lang]
	c.mu.RUnlock()
	if entry.policy == nil || now.Sub(entry.loadedAt) >= policyMaxAge {
		return nil
	}
	return entry.policy
}

func (c *PolicyCache) Run(ctx context.Context) {
	ticker := time.NewTicker(policyRefresh)
	defer ticker.Stop()
	for ctx.Err() == nil {
		c.Refresh(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *PolicyCache) Refresh(ctx context.Context) {
	var pending sync.WaitGroup
	for _, lang := range languages {
		pending.Go(func() { c.refreshLanguage(ctx, lang) })
	}
	pending.Wait()
}

func (c *PolicyCache) refreshLanguage(ctx context.Context, lang string) {
	policy, err := c.client.PublicPolicy(ctx, lang)
	if ctx.Err() != nil {
		return
	}
	var response *ResponseError
	switch {
	case err == nil:
		c.mu.Lock()
		c.byLang[lang] = policyEntry{policy, time.Now()}
		c.mu.Unlock()
	case errors.As(err, &response) && (response.Status == http.StatusNotFound || response.Status == http.StatusGone):
		// The endpoint can be switched off while the MCP process is running.
		c.mu.Lock()
		delete(c.byLang, lang)
		c.mu.Unlock()
	}
}
