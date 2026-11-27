package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/openany/openany-mcp/internal/schema"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// ModelDiscovery contains catalog hints only. It carries no execution contract.
type ModelDiscovery struct {
	Version       int            `json:"version"`
	ModelID       string         `json:"model_id"`
	SourceState   string         `json:"source_state"`
	ReadOnly      bool           `json:"read_only"`
	RelatedModels []RelatedModel `json:"related_models"`
}
type DiscoveryPricing struct {
	Status        string `json:"status"`
	Scope         string `json:"scope"`
	Currency      string `json:"currency"`
	Unit          string `json:"unit"`
	Prompt        string `json:"prompt"`
	Completion    string `json:"completion"`
	RequiresQuote bool   `json:"requires_quote"`
}
type RelatedModel struct {
	ModelID    string           `json:"model_id"`
	ProviderID string           `json:"provider_id"`
	Relation   string           `json:"relation"`
	Protocol   string           `json:"protocol"`
	Pricing    DiscoveryPricing `json:"pricing"`
}

func discoveryID(id string) bool {
	return id != "" && boundedText(id, 64) && strings.TrimSpace(id) == id
}

// ModelRecovery deliberately does not accept caller headers or a request body.
// Only the configured model origin receives this bounded, keyless metadata GET.
func (c *Client) ModelRecovery(ctx context.Context, id string) (ModelDiscovery, error) {
	var result ModelDiscovery
	if !discoveryID(id) {
		return result, fmt.Errorf("invalid model id")
	}
	u := *c.base
	u.Path = "/v1/models/recovery"
	u.RawPath = ""
	u.RawQuery = url.Values{"model": {id}}.Encode()
	timeout := min(c.timeout, 2*time.Second)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || strings.Split(resp.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return result, fmt.Errorf("model discovery unavailable")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024+1))
	if err != nil || len(raw) > 16*1024 {
		return result, fmt.Errorf("model discovery response exceeds limit")
	}
	// Decode through the shared strict parser to reject duplicate keys and invalid UTF-8.
	if !utf8.Valid(raw) {
		return result, fmt.Errorf("invalid discovery encoding")
	}
	if _, err := schema.Decode(raw); err != nil {
		return result, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&result); err != nil {
		return ModelDiscovery{}, err
	}
	if result.Version != 1 || result.ModelID != id || !result.ReadOnly || len(result.RelatedModels) > 3 {
		return ModelDiscovery{}, fmt.Errorf("invalid model discovery response")
	}
	switch result.SourceState {
	case "unknown", "available", "unsupported":
		if len(result.RelatedModels) != 0 {
			return ModelDiscovery{}, fmt.Errorf("unexpected related models")
		}
	case "unavailable":
	default:
		return ModelDiscovery{}, fmt.Errorf("invalid source state")
	}
	seen := map[string]bool{}
	for _, related := range result.RelatedModels {
		p := related.Pricing
		if !discoveryID(related.ModelID) || related.ModelID == id || seen[related.ModelID] || !publicToken.MatchString(related.ProviderID) || related.Relation != "similar_model" || related.Protocol != "openai-chat-completions" || p.Status != "known" || p.Scope != "public_catalog" || p.Currency != "USD" || p.Unit != "token" || p.RequiresQuote || !candidateAmount.MatchString(p.Prompt) || !candidateAmount.MatchString(p.Completion) {
			return ModelDiscovery{}, fmt.Errorf("invalid related model")
		}
		seen[related.ModelID] = true
	}
	return result, nil
}
