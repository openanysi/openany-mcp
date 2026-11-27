package mcpserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/account"
	"github.com/openany/openany-mcp/internal/gateway"
)

func (h *Handler) accountEnabled() bool {
	return h.options.LiteEnabled && h.options.AccountClient != nil
}

func (h *Handler) getAccount(ctx context.Context, req *mcp.CallToolRequest, lang string) *mcp.CallToolResult {
	var headers http.Header
	if req.Extra != nil {
		headers = req.Extra.Header
	}
	if result, blocked := placeholderCredentials(headers); blocked {
		result.Value["authentication"] = "invalid"
		result.Value["businessRequestStarted"] = false
		result.Value["primaryAction"] = "configure_credentials"
		return toLiteMCP(result)
	}
	policy := h.options.PolicyCache.Current(lang)
	links := onboardingLinks(policy)
	if !credentialPresent(headers) {
		return toMCP(gateway.Result{Value: map[string]any{
			"version": 1, "authentication": "missing", "businessRequestStarted": false,
			"primaryAction": "register_or_configure", "links": links, "publicBenefits": publicBenefits(policy, links),
			"setupInstructions": "Create or sign in to your OpenAny account, then put your OpenAny API key in the client's secure connector settings. Keep the key out of chat and tool inputs.",
		}})
	}

	summary, err := h.options.AccountClient.Summary(ctx, lang, headers)
	if err == nil {
		value := map[string]any{
			"version": summary.Version, "authentication": summary.Authentication, "businessRequestStarted": false,
			"primaryAction": summary.PrimaryAction, "links": summary.Links, "asOf": summary.AsOf,
			"publicBenefits": publicBenefits(summary.PublicBenefits, onboardingLinks(summary.PublicBenefits)),
			"balance":        summary.Balance, "apiKey": summary.APIKey, "signupCredit": summary.SignupCredit, "referral": summary.Referral,
		}
		return toMCP(gateway.Result{Value: value})
	}

	code, state, action := "openany_mcp_account_unavailable", "unavailable", "retry_later"
	message := "The OpenAny account summary is temporarily unavailable. Try the account lookup again or open the official account dashboard."
	var status, requestID any
	var response *account.ResponseError
	if errors.As(err, &response) {
		status = response.Status
		if response.Code != "" {
			code = response.Code
		}
		if response.RequestID != "" {
			requestID = response.RequestID
		}
		switch response.Status {
		case http.StatusUnauthorized:
			state, action, message = "invalid", "replace_credentials", rejectedKeyMessage
		case http.StatusForbidden:
			state, action = "restricted", "review_account"
			message = "This credential cannot read the account summary. Review access in the official OpenAny account dashboard."
		}
	}
	result := gateway.Failure(code, message, "account", "not_started", status, requestID)
	result.Value["authentication"] = state
	result.Value["version"] = 1
	result.Value["businessRequestStarted"] = false
	result.Value["primaryAction"] = action
	result.Value["links"] = links
	result.Value["error"].(map[string]any)["next_action"] = message
	return toMCP(result)
}
