package docs

import (
	"html"
	"strings"
	"unicode/utf8"
)

const guideCSS = `body{margin:0;background:#f6f7f4;color:#1c1c1b;font:16px/1.5 ui-sans-serif,system-ui,sans-serif}main{max-width:46rem;margin:0 auto;padding:2rem 1.25rem 4rem}h1,h2,h3{line-height:1.25}code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}pre{overflow:auto;padding:0.9rem 1rem;background:#fff;border:1px solid #d9d6cc;border-radius:8px}code{background:#efece4;padding:0.1em 0.3em;border-radius:4px}pre code{background:transparent;padding:0}table{border-collapse:collapse;width:100%;background:#fff}th,td{border:1px solid #d9d6cc;padding:0.4rem 0.6rem;text-align:left;vertical-align:top}a{color:#0b5}details{margin-top:2rem}summary{cursor:pointer;font-weight:600}`

// HTML renders a small, safe subset of the connection guide.
// It does not load scripts or remote stylesheets.
func HTML(markdown, lang string) string {
	if lang != "zh" {
		lang = "en"
	}
	title := "OpenAny MCP"
	if lang == "zh" {
		title = "OpenAny MCP 接入"
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html lang=\"")
	b.WriteString(lang)
	b.WriteString("\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>")
	b.WriteString(html.EscapeString(title))
	b.WriteString("</title><style>")
	b.WriteString(guideCSS)
	b.WriteString("</style></head><body><main>")
	b.WriteString(renderMarkdown(markdown, lang))
	b.WriteString("</main></body></html>")
	return b.String()
}

func renderMarkdown(markdown, lang string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var b strings.Builder
	var para []string
	details := false
	flush := func() {
		if len(para) == 0 {
			return
		}
		b.WriteString("<p>")
		b.WriteString(inline(strings.Join(para, " ")))
		b.WriteString("</p>")
		para = nil
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") {
			flush()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, lines[i])
			}
			b.WriteString("<pre><code>")
			b.WriteString(html.EscapeString(strings.Join(code, "\n")))
			b.WriteString("</code></pre>")
			continue
		}
		if isTableRow(trim) && i+1 < len(lines) && isTableRule(strings.TrimSpace(lines[i+1])) {
			flush()
			header := splitRow(trim)
			i += 2
			var rows [][]string
			for i < len(lines) && isTableRow(strings.TrimSpace(lines[i])) {
				rows = append(rows, splitRow(strings.TrimSpace(lines[i])))
				i++
			}
			i--
			b.WriteString("<table><thead><tr>")
			for _, cell := range header {
				b.WriteString("<th>")
				b.WriteString(inline(cell))
				b.WriteString("</th>")
			}
			b.WriteString("</tr></thead><tbody>")
			for _, row := range rows {
				b.WriteString("<tr>")
				for _, cell := range row {
					b.WriteString("<td>")
					b.WriteString(inline(cell))
					b.WriteString("</td>")
				}
				b.WriteString("</tr>")
			}
			b.WriteString("</tbody></table>")
			continue
		}
		if trim == "<!-- developer-docs -->" {
			flush()
			summary := "Developer docs"
			if lang == "zh" {
				summary = "开发者文档"
			}
			b.WriteString("<details><summary>")
			b.WriteString(html.EscapeString(summary))
			b.WriteString("</summary>")
			details = true
			continue
		}
		if trim == "" {
			flush()
			continue
		}
		if heading, ok := strings.CutPrefix(trim, "### "); ok {
			flush()
			b.WriteString("<h3>")
			b.WriteString(inline(heading))
			b.WriteString("</h3>")
			continue
		}
		if heading, ok := strings.CutPrefix(trim, "## "); ok {
			flush()
			b.WriteString("<h2>")
			b.WriteString(inline(heading))
			b.WriteString("</h2>")
			continue
		}
		if heading, ok := strings.CutPrefix(trim, "# "); ok {
			flush()
			b.WriteString("<h1>")
			b.WriteString(inline(heading))
			b.WriteString("</h1>")
			continue
		}
		if strings.HasPrefix(trim, "- ") {
			flush()
			b.WriteString("<ul>")
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "- ") {
				b.WriteString("<li>")
				b.WriteString(inline(strings.TrimSpace(lines[i])[2:]))
				b.WriteString("</li>")
				i++
			}
			i--
			b.WriteString("</ul>")
			continue
		}
		if n, rest, ok := numbered(trim); ok {
			flush()
			b.WriteString("<ol>")
			b.WriteString("<li>")
			b.WriteString(inline(rest))
			b.WriteString("</li>")
			_ = n
			for i+1 < len(lines) {
				next := strings.TrimSpace(lines[i+1])
				if _, rest, ok := numbered(next); ok {
					i++
					b.WriteString("<li>")
					b.WriteString(inline(rest))
					b.WriteString("</li>")
					continue
				}
				break
			}
			b.WriteString("</ol>")
			continue
		}
		para = append(para, trim)
	}
	flush()
	if details {
		b.WriteString("</details>")
	}
	return b.String()
}

func isTableRow(line string) bool {
	return strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") && strings.Count(line, "|") >= 2
}

func isTableRule(line string) bool {
	if !isTableRow(line) {
		return false
	}
	for _, cell := range splitRow(line) {
		cell = strings.TrimSpace(cell)
		if cell == "" || strings.Trim(cell, "-:") != "" {
			return false
		}
	}
	return true
}

func splitRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func numbered(line string) (string, string, bool) {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i+2 > len(line) || line[i] != '.' || line[i+1] != ' ' {
		return "", "", false
	}
	return line[:i], line[i+2:], true
}

func inline(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '`' {
			end := strings.IndexByte(s[i+1:], '`')
			if end >= 0 {
				b.WriteString("<code>")
				b.WriteString(html.EscapeString(s[i+1 : i+1+end]))
				b.WriteString("</code>")
				i += end + 2
				continue
			}
		}
		if strings.HasPrefix(s[i:], "**") {
			end := strings.Index(s[i+2:], "**")
			if end >= 0 {
				b.WriteString("<strong>")
				b.WriteString(html.EscapeString(s[i+2 : i+2+end]))
				b.WriteString("</strong>")
				i += end + 4
				continue
			}
		}
		if s[i] == '[' {
			if label, href, next, ok := linkAt(s[i:]); ok {
				b.WriteString("<a href=\"")
				b.WriteString(html.EscapeString(href))
				b.WriteString("\">")
				b.WriteString(html.EscapeString(label))
				b.WriteString("</a>")
				i += next
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteString(html.EscapeString(string(r)))
		i += size
	}
	return b.String()
}

func linkAt(s string) (label, href string, next int, ok bool) {
	endLabel := strings.IndexByte(s, ']')
	if endLabel < 2 || endLabel+1 >= len(s) || s[endLabel+1] != '(' {
		return "", "", 0, false
	}
	endHref := strings.IndexByte(s[endLabel+2:], ')')
	if endHref < 0 {
		return "", "", 0, false
	}
	href = s[endLabel+2 : endLabel+2+endHref]
	if !safeHref(href) {
		return "", "", 0, false
	}
	return s[1:endLabel], href, endLabel + 2 + endHref + 1, true
}

func safeHref(href string) bool {
	lower := strings.ToLower(strings.TrimSpace(href))
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "/") || strings.HasPrefix(lower, "#")
}
