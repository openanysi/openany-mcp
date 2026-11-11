package mcpserver

import (
	"net/http"
	"strconv"
	"strings"

	publicdocs "github.com/openany/openany-mcp/docs"
)

func (h *Handler) serveGuide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	lang := guideLanguage(r)
	body := publicdocs.MCPEnglish
	if lang == "zh" {
		body = publicdocs.MCP
	}
	w.Header().Set("Content-Language", lang)
	w.Header().Set("Vary", "Accept, Accept-Language")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if wantsHTML(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			w.Write([]byte(publicdocs.HTML(body, lang)))
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if r.Method == http.MethodGet {
		w.Write([]byte(body))
	}
}

func wantsHTML(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html")
}

func guideLanguage(r *http.Request) string {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang"))) {
	case "en", "en-us", "en-gb":
		return "en"
	case "zh", "zh-cn", "zh-hans", "zh-tw", "zh-hk":
		return "zh"
	}
	if preferChinese(r.Header.Get("Accept-Language")) {
		return "zh"
	}
	return "en"
}

func preferChinese(header string) bool {
	zh, en := -1.0, -1.0
	for _, part := range strings.Split(header, ",") {
		tag, q, ok := languageTag(part)
		if !ok || tag == "*" {
			continue
		}
		switch {
		case tag == "zh" || strings.HasPrefix(tag, "zh-"):
			if q > zh {
				zh = q
			}
		case tag == "en" || strings.HasPrefix(tag, "en-"):
			if q > en {
				en = q
			}
		}
	}
	return zh >= 0 && zh > en
}

func languageTag(part string) (string, float64, bool) {
	part = strings.TrimSpace(part)
	if part == "" {
		return "", 0, false
	}
	tag := part
	q := 1.0
	if i := strings.IndexByte(part, ';'); i >= 0 {
		tag = strings.TrimSpace(part[:i])
		for _, param := range strings.Split(part[i+1:], ";") {
			param = strings.TrimSpace(param)
			name, value, ok := strings.Cut(param, "=")
			if ok && strings.EqualFold(strings.TrimSpace(name), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					return "", 0, false
				}
				q = parsed
			}
		}
	}
	return strings.ToLower(tag), q, tag != ""
}
