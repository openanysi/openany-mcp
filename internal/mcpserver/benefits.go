package mcpserver

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/account"
)

func onboardingLinks(policy *account.Policy) account.Links {
	var links account.Links
	if policy != nil {
		links = policy.Links
	}
	return account.Links{
		Account:       cmp.Or(links.Account, keyPageURL),
		Register:      cmp.Or(links.Register, keyPageURL),
		Setup:         cmp.Or(links.Setup, "https://openany.si/setup.md"),
		TopUp:         cmp.Or(links.TopUp, "https://openany.si/dashboard/?page=credits"),
		PublicPolicy:  cmp.Or(links.PublicPolicy, account.PublicURL),
		ReferralRules: cmp.Or(links.ReferralRules, "https://openany.si/dashboard/?page=rewards&rewardsTab=referral"),
	}
}

func publicBenefits(policy *account.Policy, links account.Links) any {
	if policy != nil {
		return policy
	}
	return map[string]any{"version": 1, "status": "unavailable", "links": links}
}

func (h *Handler) addBenefits(value map[string]any) map[string]any {
	if !h.options.BenefitsEnabled {
		return value
	}
	policy := h.options.PolicyCache.Current("en")
	links := onboardingLinks(policy)
	benefits := map[string]any{
		"publicPolicy":  publicBenefits(policy, links),
		"rulesUrl":      links.PublicPolicy,
		"applicability": "OpenAny account benefits are subject to eligibility and are separate from API and model prices.",
	}
	action := map[string]string{"url": links.Account}
	if h.accountEnabled() {
		action["tool"] = "get_account"
	}
	benefits["checkAccount"] = action
	value["benefits"] = benefits
	return value
}

func (h *Handler) benefitsInstructions() string {
	if !h.options.BenefitsEnabled {
		return ""
	}
	return " See current trial-credit and referral terms at " + account.PublicURL + "."
}

func (h *Handler) accountInstructions() string {
	if !h.accountEnabled() {
		return ""
	}
	return " Use get_account when the user asks about account status or registration. It reads the account without starting a business call."
}

func trialDescription(policy *account.Policy) string {
	amounts := policy.TrialAmounts()
	if len(amounts) == 0 {
		return ""
	}
	if common := policy.TrialAmount(); common > 0 {
		return "Eligible Agent newcomers automatically receive " + trialUSD(common) + " in credit."
	}
	parts := make([]string, 0, len(amounts))
	for _, channel := range policy.SignupCredit.EligibleChannels {
		var label string
		switch channel {
		case "agent_wallet":
			label = "Agent wallet registration"
		case "cli_email":
			label = "CLI email registration"
		default:
			return "Eligible Agent newcomers automatically receive channel-specific trial credit. See the current trial-credit terms."
		}
		parts = append(parts, trialUSD(amounts[channel])+" for "+label)
	}
	return "Eligible Agent newcomers automatically receive trial credit: " + strings.Join(parts, "; ") + "."
}

func trialUSD(amount int64) string {
	usd := fmt.Sprintf("%d.%06d", amount/1_000_000, amount%1_000_000)
	usd = strings.TrimRight(usd, "0")
	dot := strings.IndexByte(usd, '.')
	if decimals := len(usd) - dot - 1; decimals < 2 {
		usd += strings.Repeat("0", 2-decimals)
	}
	return "$" + usd
}

// The policy revision selects only display metadata. Tools and cursors are
// built from the catalog and each response owns its tool copies.
func (h *Handler) describeBenefits(result *mcp.ListToolsResult, lite bool) *mcp.ListToolsResult {
	if !h.options.BenefitsEnabled {
		return result
	}
	policy := h.options.PolicyCache.Current("en")
	trial := trialDescription(policy)
	referral := ""
	if policy != nil && policy.ReferralReward.Automatic && policy.ReferralReward.Recipient == "inviter" {
		referral = "Qualifying referrals automatically credit the inviter."
	}
	view := *result
	view.Tools = make([]*mcp.Tool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		description := tool.Description
		entry := !lite || tool.Name == "openany_overview" || tool.Name == "get_account"
		if entry || tool.Name == "search_apis" || tool.Name == "get_api" {
			summary := trial
			if entry {
				summary = strings.TrimSpace(trial + " " + referral)
			}
			if summary == "" {
				summary = "See current trial-credit and referral terms."
			}
			if lite {
				description += " " + summary
			} else {
				description += "\n" + summary + " See " + account.PublicURL + "."
			}
		}
		copy := *tool
		copy.Description = description
		view.Tools = append(view.Tools, &copy)
	}
	return &view
}
