package transport

import (
	"encoding/json"
	"fmt"
	"github.com/openany/openany-mcp/internal/model"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type Encoded struct {
	Path    string
	Query   string
	Body    []byte
	Headers http.Header
}

var safeHeader = regexp.MustCompile(`^[!#$%&'*+.^_` + "`" + `|~0-9a-zA-Z-]+$`)

func ReservedHeader(s string) bool {
	n := strings.ToLower(s)
	switch n {
	case "authorization", "x-api-key", "xi-api-key", "host", "cookie", "connection", "content-length", "content-type", "content-encoding", "transfer-encoding", "accept", "proxy-authorization", "proxy-connection", "te", "trailer", "upgrade":
		return true
	}
	return strings.HasPrefix(n, "mcp-") || strings.HasPrefix(n, "x-forwarded-") || strings.HasPrefix(n, "x-agent-internal")
}
func scalar(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case json.Number:
		return x.String(), nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("value has no scalar HTTP encoding")
	}
}
func Encode(r model.Request, args map[string]any) (Encoded, error) {
	e := Encoded{Path: r.PublicPath, Headers: make(http.Header)}
	if err := ValidateInputs(r.Inputs); err != nil {
		return e, err
	}
	q := url.Values{}
	body := map[string]any{}
	var root any
	hasRoot := false
	for _, in := range r.Inputs {
		v, ok := args[in.Name]
		if !ok {
			continue
		}
		switch in.Location {
		case "path":
			s, err := scalar(v)
			if err != nil {
				return e, err
			}
			if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00") || strings.Contains(s, "%") {
				return e, fmt.Errorf("unsafe path parameter")
			}
			placeholder := "{" + in.Name + "}"
			if !strings.Contains(e.Path, placeholder) {
				return e, fmt.Errorf("unknown path placeholder")
			}
			e.Path = strings.ReplaceAll(e.Path, placeholder, url.PathEscape(s))
		case "query":
			if v == nil && in.OmitNull {
				continue
			}
			if a, ok := v.([]any); ok {
				policy, exists := r.QueryArrays[in.Name]
				if !exists {
					return e, fmt.Errorf("array query encoding missing for %s", in.Name)
				}
				if len(a) == 0 && policy.Empty == "reject" {
					return e, fmt.Errorf("empty query array is not supported for %s", in.Name)
				}
				if len(a) == 0 && policy.Empty == "omit" {
					continue
				}
				parts := []string{}
				for _, item := range a {
					s, err := scalar(item)
					if err != nil {
						return e, err
					}
					parts = append(parts, s)
				}
				switch policy.Encoding {
				case "csv":
					for _, s := range parts {
						if strings.Contains(s, ",") {
							return e, fmt.Errorf("CSV query item contains delimiter")
						}
					}
					q.Set(in.Name, strings.Join(parts, ","))
				case "repeated":
					for _, s := range parts {
						q.Add(in.Name, s)
					}
					if len(parts) == 0 {
						q.Set(in.Name, "")
					}
				case "json":
					b, err := json.Marshal(a)
					if err != nil {
						return e, err
					}
					q.Set(in.Name, string(b))
				default:
					return e, fmt.Errorf("unknown array encoding")
				}
			} else {
				s, err := scalar(v)
				if err != nil {
					return e, err
				}
				q.Set(in.Name, s)
			}
		case "header":
			if !safeHeader.MatchString(in.Name) || ReservedHeader(in.Name) {
				return e, fmt.Errorf("reserved or invalid business header")
			}
			s, err := scalar(v)
			if err != nil {
				return e, err
			}
			if strings.ContainsAny(s, "\r\n\x00") {
				return e, fmt.Errorf("invalid header value")
			}
			e.Headers.Set(in.Name, s)
		case "body":
			if in.Pointer != nil && *in.Pointer == "" {
				if hasRoot || len(body) > 0 {
					return e, fmt.Errorf("conflicting root body")
				}
				root = v
				hasRoot = true
			} else {
				if hasRoot {
					return e, fmt.Errorf("conflicting body inputs")
				}
				p := "/" + strings.ReplaceAll(strings.ReplaceAll(in.Name, "~", "~0"), "/", "~1")
				if in.Pointer != nil {
					p = *in.Pointer
				}
				if err := insert(body, p, v); err != nil {
					return e, err
				}
			}
		default:
			return e, fmt.Errorf("unsupported input location")
		}
	}
	if strings.ContainsAny(e.Path, "{}") {
		return e, fmt.Errorf("missing path parameter")
	}
	e.Query = q.Encode()
	if r.Method != "GET" && r.RequestContentType == "application/json" {
		var value any = body
		if hasRoot {
			value = root
		}
		var err error
		e.Body, err = json.Marshal(value)
		if err != nil {
			return e, err
		}
		e.Headers.Set("Content-Type", "application/json")
	}
	return e, nil
}
func insert(body map[string]any, p string, value any) error {
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("invalid JSON pointer")
	}
	parts := strings.Split(p[1:], "/")
	node := body
	for i, s := range parts {
		if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(s, "~0", ""), "~1", ""), "~") {
			return fmt.Errorf("invalid pointer escape")
		}
		s = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
		if i == len(parts)-1 {
			if _, exists := node[s]; exists {
				return fmt.Errorf("overlapping body pointers")
			}
			node[s] = value
			return nil
		}
		existing, ok := node[s]
		if !ok {
			next := map[string]any{}
			node[s] = next
			node = next
		} else {
			next, ok := existing.(map[string]any)
			if !ok {
				return fmt.Errorf("conflicting body pointer")
			}
			node = next
		}
	}
	return nil
}

// ValidateInputs rejects aliases and ancestor/descendant targets before any value is inserted.
func ValidateInputs(inputs []model.Parameter) error {
	paths := [][]string{}
	for _, in := range inputs {
		if in.Location != "body" {
			continue
		}
		var parts []string
		if in.Pointer == nil {
			parts = []string{in.Name}
		} else if *in.Pointer != "" {
			if !strings.HasPrefix(*in.Pointer, "/") {
				return fmt.Errorf("invalid JSON pointer")
			}
			for _, part := range strings.Split((*in.Pointer)[1:], "/") {
				if strings.Contains(strings.ReplaceAll(strings.ReplaceAll(part, "~0", ""), "~1", ""), "~") {
					return fmt.Errorf("invalid pointer escape")
				}
				parts = append(parts, strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~"))
			}
		}
		for _, previous := range paths {
			n := len(parts)
			if len(previous) < n {
				n = len(previous)
			}
			overlap := true
			for i := 0; i < n; i++ {
				if parts[i] != previous[i] {
					overlap = false
					break
				}
			}
			if overlap {
				return fmt.Errorf("overlapping body pointers")
			}
		}
		paths = append(paths, parts)
	}
	return nil
}
