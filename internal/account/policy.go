package account

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	Automatic               bool              `json:"automatic"`
	Status                  string            `json:"status"`
	AmountMicroUSD          string            `json:"amountMicroUsd"`
	AmountMicroUSDByChannel map[string]string `json:"amountMicroUsdByChannel,omitempty"`
	Currency                string            `json:"currency"`
	EligibleChannels        []string          `json:"eligibleChannels"`
	RulesURL                string            `json:"rulesUrl"`
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

// TrialAmounts returns an owned, validated view of the eligible channels. Older
// policies omit the channel map when all channels share AmountMicroUSD.
func (p *Policy) TrialAmounts() map[string]int64 {
	if !p.valid() || p.SignupCredit.Status != "enabled" || !p.SignupCredit.Automatic ||
		p.SignupCredit.Currency != "USD" || len(p.SignupCredit.EligibleChannels) == 0 {
		return nil
	}
	parse := func(raw string) (int64, bool) {
		amount, err := strconv.ParseInt(raw, 10, 64)
		return amount, err == nil && amount > 0 && strconv.FormatInt(amount, 10) == raw
	}
	primary, ok := parse(p.SignupCredit.AmountMicroUSD)
	if !ok {
		return nil
	}
	byChannel := p.SignupCredit.AmountMicroUSDByChannel
	if byChannel != nil && len(byChannel) != len(p.SignupCredit.EligibleChannels) {
		return nil
	}
	amounts := make(map[string]int64, len(p.SignupCredit.EligibleChannels))
	for i, channel := range p.SignupCredit.EligibleChannels {
		if channel == "" || strings.TrimSpace(channel) != channel {
			return nil
		}
		if _, duplicate := amounts[channel]; duplicate {
			return nil
		}
		amount := primary
		if byChannel != nil {
			amount, ok = parse(byChannel[channel])
			if !ok || i == 0 && amount != primary {
				return nil
			}
		}
		amounts[channel] = amount
	}
	return amounts
}

// TrialAmount is a universal offer only when every eligible channel agrees.
func (p *Policy) TrialAmount() int64 {
	var common int64
	for _, amount := range p.TrialAmounts() {
		if common != 0 && common != amount {
			return 0
		}
		common = amount
	}
	return common
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
