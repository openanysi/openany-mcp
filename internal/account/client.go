package account

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openany/openany-mcp/internal/gateway"
)

const (
	policyPath  = "/api/v1/agent-onboarding"
	accountPath = "/api/v1/agent-api-account/onboarding"
	maxBody     = 64 << 10
)

var credentialHeaders = [...]string{"Authorization", "X-Api-Key", "Xi-Api-Key"}

type Client struct {
	origin string
	http   *http.Client
}

type ResponseError struct {
	Status    int
	Code      string `json:"code"`
	RequestID string `json:"requestId"`
}

func (*ResponseError) Error() string { return "account endpoint rejected the request" }

func ParseOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.ForceQuery || u.RawQuery != "" ||
		strings.Contains(raw, "#") || u.RawPath != "" || (u.Path != "" && u.Path != "/") ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return "", errors.New("invalid MCP_ACCOUNT_BASE_URL")
	}
	u.Path = ""
	return u.String(), nil
}

func NewClient(raw string) (*Client, error) {
	origin, err := ParseOrigin(raw)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 500 * time.Millisecond}).DialContext,
		TLSHandshakeTimeout: 500 * time.Millisecond,
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     time.Minute,
	}
	return &Client{origin: origin, http: &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

var languages = [...]string{"en", "zh", "ja", "ar"}

func (c *Client) get(ctx context.Context, path, lang string, headers http.Header, dst any) error {
	if lang == "" {
		lang = "en"
	}
	switch lang {
	case "en", "zh", "ja", "ar":
	default:
		return errors.New("invalid account language")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.origin+path+"?lang="+lang, nil)
	if err != nil {
		return errors.New("invalid account request")
	}
	req.Header.Set("Accept", "application/json")
	for _, name := range credentialHeaders {
		for _, value := range headers.Values(name) {
			if strings.TrimSpace(value) == "" {
				continue
			}
			if gateway.CredentialPlaceholder(name, value) {
				return errors.New("unexpanded account credential")
			}
			if name == "Authorization" {
				value = gateway.AuthorizationValue(value)
			}
			req.Header.Add(name, value)
		}
	}
	res, err := c.http.Do(req)
	if err != nil {
		return errors.New("account endpoint unavailable")
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil || len(raw) > maxBody {
		return errors.New("account endpoint response exceeded limits or was interrupted")
	}
	if res.StatusCode != http.StatusOK {
		failure := &ResponseError{}
		_ = json.Unmarshal(raw, failure)
		failure.Status = res.StatusCode
		if failure.RequestID == "" {
			failure.RequestID = res.Header.Get("X-Request-Id")
		}
		return failure
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || json.Unmarshal(envelope.Data, dst) != nil {
		return errors.New("invalid account endpoint response")
	}
	return nil
}

func (c *Client) PublicPolicy(ctx context.Context, lang string) (*Policy, error) {
	if lang == "" {
		lang = "en"
	}
	var policy Policy
	if err := c.get(ctx, policyPath, lang, nil, &policy); err != nil {
		return nil, err
	}
	if !policy.valid() || (policy.Lang != "" && policy.Lang != lang) {
		return nil, errors.New("invalid public benefits policy")
	}
	return &policy, nil
}

type Summary struct {
	Version                int         `json:"version"`
	Authentication         string      `json:"authentication"`
	BusinessRequestStarted *bool       `json:"businessRequestStarted"`
	AsOf                   string      `json:"asOf,omitempty"`
	PrimaryAction          string      `json:"primaryAction,omitempty"`
	Links                  Links       `json:"links"`
	PublicBenefits         *Policy     `json:"publicBenefits,omitempty"`
	Balance                Balance     `json:"balance"`
	APIKey                 Allowance   `json:"apiKey"`
	SignupCredit           SignupGrant `json:"signupCredit"`
	Referral               Referral    `json:"referral"`
}

type Balance struct {
	Status            string `json:"status"`
	AvailableMicroUSD string `json:"availableMicroUsd,omitempty"`
	BalanceMicroUSD   string `json:"balanceMicroUsd,omitempty"`
	Currency          string `json:"currency,omitempty"`
	Source            string `json:"source,omitempty"`
	AsOf              string `json:"asOf,omitempty"`
}

type Allowance struct {
	Status               string          `json:"status"`
	SpendLimitMicroUSD   json.RawMessage `json:"spendLimitMicroUsd,omitempty"`
	RemainingMicroUSD    json.RawMessage `json:"remainingMicroUsd,omitempty"`
	MonthlyUsageMicroUSD string          `json:"monthlyUsageMicroUsd,omitempty"`
	BillingMonth         string          `json:"billingMonth,omitempty"`
	Source               string          `json:"source,omitempty"`
}

type SignupGrant struct {
	Status                string `json:"status"`
	GrantedAmountMicroUSD string `json:"grantedAmountMicroUsd,omitempty"`
	GrantedAt             string `json:"grantedAt,omitempty"`
}

type Referral struct {
	Status         string `json:"status"`
	RulesURL       string `json:"rulesUrl,omitempty"`
	DashboardURL   string `json:"dashboardUrl,omitempty"`
	ReferralLink   string `json:"referralLink,omitempty"`
	EarnedMicroUSD string `json:"earnedMicroUsd,omitempty"`
}

func (c *Client) Summary(ctx context.Context, lang string, headers http.Header) (*Summary, error) {
	var summary Summary
	if err := c.get(ctx, accountPath, lang, headers, &summary); err != nil {
		return nil, err
	}
	if summary.Version != 1 || summary.Authentication != "authenticated" ||
		summary.BusinessRequestStarted == nil || *summary.BusinessRequestStarted {
		return nil, errors.New("invalid authenticated account summary")
	}
	if summary.Referral.Status != "available" {
		summary.Referral.ReferralLink = ""
		summary.Referral.EarnedMicroUSD = ""
	}
	if !summary.PublicBenefits.valid() {
		summary.PublicBenefits = nil
	}
	return &summary, nil
}
