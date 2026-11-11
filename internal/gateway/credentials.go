package gateway

import (
	"net/http"
	"strings"
	"unicode"
)

const KeyPageURL = "https://openany.si/dashboard?page=api-keys"

const PlaceholderCredentialCode = "openany_mcp_credentials_placeholder"

const PlaceholderCredentialMessage = "The Authorization header contains an unexpanded placeholder. The environment variable was not set or expanded by the client. Set OPENANY_API_KEY in the client's environment or secret settings and restart the client. Do not paste the key into the chat or prompt. Create a key at " + KeyPageURL + "."

// AuthorizationValue adds the Bearer scheme when the value is a raw key.
// A value that already has a scheme, including case-insensitive "bearer",
// is forwarded unchanged. X-Api-Key and Xi-Api-Key are not rewritten.
func AuthorizationValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.Contains(trimmed, " ") {
		return value
	}
	return "Bearer " + trimmed
}

// CredentialPlaceholder reports an unexpanded template in a caller credential.
// The header value is never included in errors built from this check.
func CredentialPlaceholder(name, value string) bool {
	return placeholderMaterial(credentialMaterial(name, value))
}

func credentialMaterial(name, value string) string {
	value = strings.TrimSpace(value)
	if !strings.EqualFold(name, "Authorization") {
		return value
	}
	scheme, rest, ok := strings.Cut(value, " ")
	if ok && strings.EqualFold(scheme, "Bearer") {
		return strings.TrimSpace(rest)
	}
	return value
}

func placeholderMaterial(material string) bool {
	if material == "" {
		return false
	}
	if strings.Contains(material, "${") {
		return true
	}
	if strings.HasPrefix(material, "$") && ident(material[1:]) {
		return true
	}
	if len(material) > 2 && strings.HasPrefix(material, "<") && strings.HasSuffix(material, ">") && !strings.ContainsAny(material[1:len(material)-1], "<> \t") {
		return true
	}
	switch strings.ToUpper(material) {
	case "YOUR_API_KEY", "YOUR_OPENANY_KEY", "YOUR_OPENANY_API_KEY", "OPENANY_API_KEY":
		return true
	default:
		return false
	}
}

func ident(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func applyCallerCredentials(dst http.Header, src http.Header) Result {
	if src == nil {
		return Result{}
	}
	for _, name := range []string{"Authorization", "X-Api-Key", "Xi-Api-Key"} {
		values := src.Values(name)
		if len(values) == 0 {
			continue
		}
		for _, value := range values {
			if CredentialPlaceholder(name, value) {
				result := Failure(PlaceholderCredentialCode, PlaceholderCredentialMessage, "validate", "not_started", nil, nil)
				result.Value["key_url"] = KeyPageURL
				result.Value["error"].(map[string]any)["next_action"] = PlaceholderCredentialMessage
				return result
			}
		}
		copied := append([]string(nil), values...)
		if strings.EqualFold(name, "Authorization") {
			for i, value := range copied {
				copied[i] = AuthorizationValue(value)
			}
		}
		dst[name] = copied
	}
	return Result{}
}
