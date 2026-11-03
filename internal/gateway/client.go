package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/schema"
	"github.com/openany/openany-mcp/internal/transport"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Result struct {
	Value   map[string]any
	IsError bool
}
type Client struct {
	base        *url.URL
	http        *http.Client
	timeout     time.Duration
	maxResponse int64
	maxResult   int64
}

func New(base string, timeout, connect time.Duration, maxResponse, maxResult int64) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("invalid gateway origin")
	}
	p := new(http.Protocols)
	p.SetHTTP1(true)
	tr := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: connect}).DialContext, DisableKeepAlives: true, Protocols: p, TLSHandshakeTimeout: connect}
	h := &http.Client{Transport: tr, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &Client{u, h, timeout, maxResponse, maxResult}, nil
}
func Failure(code, message, stage, state string, status any, id any) Result {
	return Result{map[string]any{"http_status": status, "request_id": id, "error": map[string]any{"code": code, "message": message, "stage": stage, "execution_state": state}}, true}
}
func (c *Client) Call(ctx context.Context, r model.Record, o model.Override, args map[string]any, headers http.Header) Result {
	if o.ResponseMode == "unsupported" {
		return Failure("openany_mcp_unsupported_response_mode", "This API response mode is not supported.", "validate", "not_started", nil, nil)
	}
	encoded, err := transport.Encode(r, o, args)
	if err != nil {
		return Failure("openany_mcp_invalid_input", "Parameters cannot be encoded for this API.", "validate", "not_started", nil, nil)
	}
	// Keep escaped path segments intact; never resolve a client-controlled URL.
	u := *c.base
	path, err := url.PathUnescape(encoded.Path)
	if err != nil {
		return Failure("openany_mcp_invalid_input", "Invalid path encoding.", "validate", "not_started", nil, nil)
	}
	u.Path = path
	u.RawPath = encoded.Path
	u.RawQuery = encoded.Query
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, r.Method, u.String(), strings.NewReader(string(encoded.Body)))
	if err != nil {
		return Failure("openany_mcp_invalid_input", "Unable to construct request.", "validate", "not_started", nil, nil)
	}
	req.GetBody = nil
	req.Header = encoded.Headers.Clone()
	for _, name := range []string{"Authorization", "X-Api-Key", "Xi-Api-Key"} {
		if values := headers.Values(name); len(values) > 0 {
			req.Header[name] = append([]string(nil), values...)
		}
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Failure("openany_mcp_gateway_transport", "Gateway connection failed or timed out; execution and charging may have occurred.", "gateway", "unknown", nil, nil)
	}
	defer resp.Body.Close()
	var id any
	if value := resp.Header.Get("X-Request-ID"); value != "" {
		id = value
	} else if value := resp.Header.Get("X-OpenAny-Request-ID"); value != "" {
		id = value
	}
	media := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if media != "application/json" && !strings.HasSuffix(media, "+json") {
		return Failure("openany_mcp_unsupported_response", "Gateway did not return a finite JSON response.", "gateway", "unknown", resp.StatusCode, id)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponse+1))
	if err != nil || int64(len(raw)) > c.maxResponse {
		return Failure("openany_mcp_response_unavailable", "Gateway response could not be read within limits; execution may have completed.", "gateway", "unknown", resp.StatusCode, id)
	}

	data, err := schema.Decode(raw)
	if err != nil {
		return Failure("openany_mcp_invalid_response", "Gateway returned invalid JSON.", "gateway", "unknown", resp.StatusCode, id)
	}
	if obj, ok := data.(map[string]any); ok && obj["type"] == "openany_error" {
		code, _ := obj["code"].(string)
		if code == "" {
			code = "openany_mcp_gateway_error"
		}
		return Failure(code, "Gateway rejected or failed the request; inspect the request ID.", "gateway", "reported_failure", resp.StatusCode, id)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Failure("openany_mcp_gateway_http_error", "Gateway returned an unsuccessful HTTP status.", "gateway", "reported_failure", resp.StatusCode, id)
	}
	result := Result{Value: map[string]any{"http_status": resp.StatusCode, "request_id": id, "content_type": media, "data": data}}
	b, err := json.Marshal(result.Value)
	escaped, escapeErr := json.Marshal(string(b))
	if err != nil || escapeErr != nil || int64(len(b)+len(escaped))+1024 > c.maxResult {
		return Failure("openany_mcp_result_too_large", "Result exceeds MCP output limit; execution completed.", "result", "completed", resp.StatusCode, id)
	}
	return result
}
func (c *Client) Close() { c.http.CloseIdleConnections() }
