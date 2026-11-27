package gateway

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var publicToken = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

// RecoveryActionText renders platform actions without treating upstream prose as instructions.
func RecoveryActionText(action, target string) string {
	if !officialRecoveryTarget(target) {
		return ""
	}
	text := map[string]string{
		"configure_credentials":  "Configure an existing OpenAny key, or register if you are new, using secure client settings. ",
		"replace_credentials":    "Replace the invalid, expired or revoked key in secure client settings. ",
		"register_or_configure":  "Configure an existing account key, or complete registration and verify the actual credited balance. ",
		"top_up":                 "Top up the account within existing payment authorization, verify crediting, then review the original request before continuing. ",
		"review_key_limit":       "Review the API key spending limit; adding account funds alone may not resolve this limit. ",
		"correct_input":          "Correct the request using its current schema and examples before making another call. ",
		"check_original_request": "Do not retry automatically. Check the original request or task and charging state first. ",
		"retry_later":            "Wait before deciding on another call; check the original execution and charging state first. ",
		"contact_support":        "Keep the request ID and contact OpenAny support. ",
	}[action]
	if text == "" {
		return ""
	}
	result := text + target
	switch action {
	case "configure_credentials", "replace_credentials", "register_or_configure", "top_up", "review_key_limit":
		result += " Current account benefit rules: https://openany.si/api/v1/agent-onboarding."
	}
	return result
}

func officialRecoveryTarget(target string) bool {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" || u.Host != "openany.si" || u.User != nil || u.Fragment != "" {
		return false
	}
	if (u.Path == "/docs" || u.Path == "/docs/") && u.RawQuery == "" {
		return true
	}
	if u.Path != "/dashboard" && u.Path != "/dashboard/" {
		return false
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q) != 1 || len(q["page"]) != 1 {
		return false
	}
	switch q.Get("page") {
	case "api-keys", "credits", "usage":
		return true
	}
	return false
}

func boundedRecovery(value any) map[string]any {
	r, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	version, _ := r["version"].(json.Number)
	if version != "1" {
		return nil
	}
	stringField := func(key string) string { x, _ := r[key].(string); return x }
	action, target := stringField("primary_action"), stringField("next_step_target")
	if RecoveryActionText(action, target) == "" {
		return nil
	}
	state, charge := stringField("execution_state"), stringField("charge_state")
	if !strings.Contains("|not_started|started|completed|failed|unknown|", "|"+state+"|") || state == "" {
		return nil
	}
	if !strings.Contains("|not_charged|charged|pending|unknown|", "|"+charge+"|") || charge == "" {
		return nil
	}
	reason := stringField("reason")
	if !publicToken.MatchString(reason) {
		return nil
	}
	out := map[string]any{"version": version, "reason": reason, "execution_state": state, "charge_state": charge, "primary_action": action, "next_step_target": target}
	if id := stringField("recovery_id"); publicToken.MatchString(id) {
		out["recovery_id"] = id
	}
	return out
}

// Old gateways may preserve a provider's JSON under details. A recognized URL
// alone does not make that provider-selected action a platform instruction.
func recoveryMatchesCode(code string, r map[string]any) bool {
	reason, _ := r["reason"].(string)
	state, _ := r["execution_state"].(string)
	action, _ := r["primary_action"].(string)
	target, _ := r["next_step_target"].(string)
	var expectedReason, expectedAction, expectedPage string
	switch code {
	case "openany_authentication_missing":
		expectedReason, expectedAction, expectedPage = "missing_credentials", "configure_credentials", "api-keys"
	case "openany_authentication_invalid", "openany_key_invalid":
		expectedReason, expectedAction, expectedPage = "invalid_credentials", "replace_credentials", "api-keys"
		// The Agent API historically uses authentication_invalid for missing keys.
		if code == "openany_authentication_invalid" && reason == "missing_credentials" {
			expectedReason, expectedAction = "missing_credentials", "configure_credentials"
		}
	case "openany_insufficient_balance":
		expectedReason, expectedAction, expectedPage = "insufficient_balance", "top_up", "credits"
	case "openany_spend_limit_exceeded":
		expectedReason, expectedAction, expectedPage = "spend_limit_exceeded", "review_key_limit", "api-keys"
	case "openany_invalid_request", "openany_invalid_json", "openany_payload_too_large", "openany_unsupported_media_type", "openany_missing_parameter", "openany_invalid_parameter", "openany_unsupported_parameter", "openany_route_incompatible", "openany_endpoint_not_found", "openany_method_not_allowed":
		expectedReason, expectedAction = "invalid_parameters", "correct_input"
	case "openany_not_found", "openany_task_not_found", "openany_duplicate_request", "openany_rate_limited", "openany_usage_unavailable", "openany_billing_unavailable", "openany_service_unavailable", "openany_models_unavailable", "openany_model_not_available", "openany_image_price_not_available", "openany_no_upstream_available", "openany_route_unavailable", "openany_internal_error":
		expectedReason, expectedAction, expectedPage = "request_outcome_unknown", "check_original_request", "usage"
	default:
		if !strings.HasPrefix(code, "openany_upstream_") || state == "not_started" {
			return false
		}
		expectedReason, expectedAction, expectedPage = "upstream_failure", "check_original_request", "usage"
	}
	if reason != expectedReason {
		return false
	}
	if state != "not_started" {
		expectedAction, expectedPage = "check_original_request", "usage"
	}
	if action != expectedAction {
		return false
	}
	u, _ := url.Parse(target) // boundedRecovery already requires an official URL.
	if expectedPage == "" {
		return (u.Path == "/docs" || u.Path == "/docs/") && u.RawQuery == ""
	}
	return (u.Path == "/dashboard" || u.Path == "/dashboard/") && u.Query().Get("page") == expectedPage
}

func publicFailure(obj map[string]any, status int, id any) Result {
	code, _ := obj["code"].(string)
	if !strings.HasPrefix(code, "openany_") || !publicToken.MatchString(code) {
		code = "openany_mcp_gateway_error"
	}
	r := Failure(code, "Gateway rejected or failed the request; inspect the request ID.", "gateway", "reported_failure", status, id)
	e := r.Value["error"].(map[string]any)
	if cost, ok := obj["openany_cost"]; ok {
		if cost == nil {
			r.Value["openany_cost"] = nil
		} else if n, ok := cost.(json.Number); ok && regexp.MustCompile(`^(0|[1-9][0-9]{0,18})$`).MatchString(string(n)) {
			r.Value["openany_cost"] = n
		}
	}
	if details, ok := obj["details"].(map[string]any); ok {
		if recovery := boundedRecovery(details["recovery"]); recovery != nil && recoveryMatchesCode(code, recovery) {
			e["recovery"] = recovery
			e["execution_state"] = recovery["execution_state"]
			e["charge_state"] = recovery["charge_state"]
			e["message"] = RecoveryActionText(recovery["primary_action"].(string), recovery["next_step_target"].(string))
		}
	}
	return r
}
