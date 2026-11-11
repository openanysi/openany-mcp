package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/schema"
	"github.com/openany/openany-mcp/internal/transport"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
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
func (c *Client) Call(ctx context.Context, r model.Request, args map[string]any, headers http.Header) Result {
	if r.UnsupportedResponse {
		return Failure("openany_mcp_unsupported_response_mode", "This API response mode is not supported.", "validate", "not_started", nil, nil)
	}
	encoded, err := transport.Encode(r, args)
	if err != nil {
		var missing *transport.MissingArrayPolicyError
		if errors.As(err, &missing) {
			return Failure("openany_mcp_array_policy_missing", "Omit optional parameter "+missing.Parameter+": its query array serialization policy is not configured.", "validate", "not_started", nil, nil)
		}
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
	if problem := applyCallerCredentials(req.Header, headers); problem.IsError {
		return problem
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
	formatFailure := func(code, message string) Result {
		result := Failure(code, message, "gateway", "unknown", resp.StatusCode, id)
		result.Value["content_type"] = media[:min(len(media), 128)]
		return result
	}
	isJSON := media == "application/json" || strings.HasSuffix(media, "+json")
	isText := media == "text/plain" || media == "text/markdown"
	if !isJSON && !isText {
		return formatFailure("openany_mcp_unsupported_response", "Gateway response media type is unsupported; expected finite JSON, UTF-8 plain text or Markdown.")
	}
	if isText {
		_, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
			return formatFailure("openany_mcp_unsupported_response", "Gateway text response must use UTF-8 encoding.")
		}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponse+1))
	if err != nil || int64(len(raw)) > c.maxResponse {
		return formatFailure("openany_mcp_response_unavailable", "Gateway response could not be read within limits; execution may have completed.")
	}

	var data any
	if isText {
		if !utf8.Valid(raw) {
			return formatFailure("openany_mcp_invalid_response", "Gateway text response contains invalid UTF-8.")
		}
		data = string(raw)
	} else {
		data, err = schema.Decode(raw)
		if err != nil {
			return formatFailure("openany_mcp_invalid_response", "Gateway returned invalid JSON.")
		}
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
	return c.LimitResult(result)
}

// LimitResult also checks presentation metadata added by the lite endpoint.
// It never retries the upstream call or changes its execution/charging state.
func (c *Client) LimitResult(result Result) Result {
	b, err := json.Marshal(result.Value)
	escaped, escapeErr := json.Marshal(string(b))
	if err != nil || escapeErr != nil || int64(len(b)+len(escaped))+1024 > c.maxResult {
		state := "completed"
		if e, ok := result.Value["error"].(map[string]any); ok {
			if original, ok := e["execution_state"].(string); ok {
				state = original
			}
		}
		message := "Result exceeds MCP output limit; execution completed."
		if state != "completed" {
			message = "Result exceeds MCP output limit; inspect execution_state before deciding on another call."
		}
		return Failure("openany_mcp_result_too_large", message, "result", state, result.Value["http_status"], result.Value["request_id"])
	}
	return result
}

func (c *Client) PostChatCompletion(ctx context.Context, payload map[string]any, headers http.Header) Result {
	root := ""
	request := model.Request{Method: "POST", PublicPath: "/v1/chat/completions", RequestContentType: "application/json", Inputs: []model.Parameter{{Name: "body", Location: "body", Pointer: &root}}}
	return c.Call(ctx, request, map[string]any{"body": payload}, headers)
}

func (c *Client) Close() { c.http.CloseIdleConnections() }
