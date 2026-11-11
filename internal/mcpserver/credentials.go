package mcpserver

import (
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openany/openany-mcp/internal/gateway"
)

const keyPageURL = "https://openany.si/dashboard?page=api-keys"

const missingKeyMessage = "This request has no OpenAny API key. Discovery does not need a key and does not call the paid gateway. Create a key at " + keyPageURL + " and set Authorization: Bearer <key> on the MCP connection in the client's secure secret or connector settings. Do not ask the user to paste the key into the chat or prompt."

const rejectedKeyMessage = "The OpenAny API key on this MCP connection was rejected. It may be invalid, expired, or revoked. Replace it in the client's secure secret or connector settings. Do not ask the user to paste a key into the chat or prompt. Manage keys at " + keyPageURL + "."

const elicitationUnavailable = "This client advertises URL elicitation, but OpenAny has no URL-elicitation completion page and does not collect secrets with form elicitation. Configure the key in the client's secure settings. Do not paste it into chat."

func credentialPresent(headers http.Header) bool {
	if headers == nil {
		return false
	}
	for _, name := range []string{"Authorization", "X-Api-Key", "Xi-Api-Key"} {
		for _, value := range headers.Values(name) {
			if tokenPresent(name, value) {
				return true
			}
		}
	}
	return false
}

func tokenPresent(name, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if !strings.EqualFold(name, "Authorization") {
		return true
	}
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return false
	}
	if strings.EqualFold(parts[0], "Bearer") {
		return len(parts) > 1 && strings.TrimSpace(strings.Join(parts[1:], " ")) != ""
	}
	return true
}

func supportsURLElicitation(req *mcp.CallToolRequest) bool {
	if req == nil {
		return false
	}
	caps := req.ClientCapabilities()
	return caps != nil && caps.Elicitation != nil && caps.Elicitation.URL != nil
}

// missingCredentials reports a local rejection when the MCP request has no key.
// The second result is true when the caller must not call the gateway.
// URL elicitation is recognized and not used: OpenAny has no completion page,
// and form elicitation must not collect a secret.
func missingCredentials(req *mcp.CallToolRequest) (gateway.Result, bool) {
	var headers http.Header
	if req != nil && req.Extra != nil {
		headers = req.Extra.Header
	}
	if credentialPresent(headers) {
		return gateway.Result{}, false
	}
	result := gateway.Failure("openany_mcp_credentials_missing", missingKeyMessage, "validate", "not_started", nil, nil)
	result.Value["error"].(map[string]any)["next_action"] = missingKeyMessage
	result.Value["key_url"] = keyPageURL
	if supportsURLElicitation(req) {
		result.Value["elicitation"] = map[string]any{
			"mode":      "url",
			"available": false,
			"reason":    elicitationUnavailable,
		}
	}
	return result, true
}

// rejectedCredentialCode is a gateway code that means the supplied key itself
// was refused. The live gateway uses openany_key_invalid. openany_authentication_invalid
// is the earlier name. Public docs describe the same failure as an invalid,
// expired, or revoked API key without listing more machine codes, so the
// expired and revoked siblings are included. Other gateway codes stay generic.
func rejectedCredentialCode(code string) bool {
	switch code {
	case "openany_key_invalid",
		"openany_key_expired",
		"openany_key_revoked",
		"openany_authentication_invalid",
		"openany_authentication_expired",
		"openany_authentication_revoked":
		return true
	default:
		return false
	}
}

func clarifyRejectedKey(result gateway.Result) gateway.Result {
	e, ok := result.Value["error"].(map[string]any)
	code, _ := e["code"].(string)
	if !ok || !rejectedCredentialCode(code) {
		return result
	}
	e["message"] = rejectedKeyMessage
	e["next_action"] = rejectedKeyMessage
	result.Value["key_url"] = keyPageURL
	return result
}
