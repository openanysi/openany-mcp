package docs

import (
	"html"
	"strings"
	"unicode/utf8"
)

// Mirror the public site's ui-foundations tokens without loading remote assets.
const guideCSS = `:root{--ui-text-primary:#111318;--ui-text-secondary:#394354;--ui-text-muted:#5f6b7c;--ui-line:#e5e9f0;--ui-control-line:#d7dee8;--ui-soft:#f8f9fb;--ui-accent:#0047ff}*{box-sizing:border-box}body{margin:0;background:#fff;color:var(--ui-text-secondary);font:14px/24px ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;letter-spacing:0}main{max-width:960px;margin:0 auto;padding:24px 32px 64px;overflow-wrap:anywhere}.guide-header{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;padding-bottom:20px;margin-bottom:32px;border-bottom:1px solid var(--ui-line)}.brand{font-size:16px;font-weight:700;color:var(--ui-text-primary);text-decoration:none}.guide-header nav{display:flex;gap:20px;flex-wrap:wrap;font-size:12px}.guide-header [aria-current]{color:var(--ui-text-primary);font-weight:600}h1{margin:0 0 24px;color:var(--ui-text-primary);font-size:28px;line-height:36px;font-weight:600}h2{margin:32px 0 16px;padding-top:24px;border-top:1px solid var(--ui-line);color:var(--ui-text-primary);font-size:16px;line-height:24px;font-weight:600}h3{margin:24px 0 12px;color:var(--ui-text-primary);font-size:14px;line-height:24px;font-weight:600}p{margin:12px 0}li{margin:8px 0}ul,ol{padding-left:24px}a{color:var(--ui-accent);text-underline-offset:3px}a:hover{text-decoration:underline}a:focus-visible,button:focus-visible,summary:focus-visible,[tabindex]:focus-visible{outline:2px solid var(--ui-accent);outline-offset:4px}code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px;line-height:20px}code{background:var(--ui-soft);padding:2px 4px;border-radius:4px}pre{max-width:100%;overflow:auto;white-space:pre;padding:16px;background:var(--ui-soft);border:1px solid var(--ui-line);border-radius:6px}pre code{background:transparent;padding:0;overflow-wrap:normal}.table-scroll{max-width:100%;overflow-x:auto;margin:16px 0;border:1px solid var(--ui-line);border-radius:6px}table{border-collapse:collapse;width:100%;min-width:520px;background:#fff;font-size:12px;line-height:20px}th,td{border-bottom:1px solid var(--ui-line);padding:10px 12px;text-align:left;vertical-align:top}th{color:var(--ui-text-primary);font-weight:600;background:var(--ui-soft)}tbody tr:last-child td{border-bottom:0}details{margin-top:32px;border-top:1px solid var(--ui-line)}summary{cursor:pointer;color:var(--ui-text-primary);font-size:16px;font-weight:600;padding:20px 0}details>h2:first-of-type{margin-top:8px}.endpoint{margin:0 0 24px;padding:16px;background:var(--ui-soft);border:1px solid var(--ui-line);border-radius:6px}.endpoint-label{display:block;font-size:12px;color:var(--ui-text-muted);margin-bottom:8px}.endpoint-row{display:flex;align-items:center;gap:12px;flex-wrap:wrap}.endpoint-row code{padding:0;background:transparent;flex:1;min-width:0}.endpoint-links{display:flex;gap:20px;flex-wrap:wrap;margin-top:12px;font-size:12px}.copy-button{font-family:inherit;font-size:12px;line-height:20px;font-weight:500;min-height:36px;padding:7px 12px;background:#fff;color:var(--ui-text-primary);border:1px solid var(--ui-control-line);border-radius:4px;cursor:pointer}.copy-button:hover{border-color:var(--ui-accent);color:var(--ui-accent)}.copy-status{display:block;min-height:20px;margin-top:4px;font-size:12px;color:var(--ui-text-muted)}.copy-status:empty{display:none}[hidden]{display:none!important}@media(max-width:620px){main{padding:20px 20px 48px}.guide-header{margin-bottom:24px}h1{font-size:26px;line-height:34px}.copy-button{min-height:44px}.guide-header nav{gap:16px}summary{min-height:44px}}`

const clipboardScript = `<script>
const copyButton = document.querySelector('[data-copy-endpoint]');
if (copyButton && navigator.clipboard && window.isSecureContext) {
  copyButton.hidden = false;
  copyButton.addEventListener('click', async () => {
    const status = document.querySelector('[data-copy-status]');
    const zh = document.documentElement.lang === 'zh';
    copyButton.disabled = true;
    try {
      await navigator.clipboard.writeText('https://mcp.openany.si');
      status.textContent = zh ? '已复制端点' : 'Endpoint copied';
    } catch {
      status.textContent = zh ? '复制失败，请手动选择并复制端点。' : 'Copy failed. Select and copy the endpoint manually.';
    } finally {
      copyButton.disabled = false;
    }
  });
}
</script>`

// HTML renders a small, safe subset of the connection guide.
// The only script copies the public endpoint; Markdown cannot add scripts.
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
	brandGuide, copyLabel, keyLabel, endpointLabel := "Connection guide", "Copy URL", "Get API key", "Server URL / Streamable HTTP"
	if lang == "zh" {
		brandGuide, copyLabel, keyLabel, endpointLabel = "接入指南", "复制端点", "创建 API key", "服务器 URL / Streamable HTTP"
	}
	b.WriteString(`<header class="guide-header"><a class="brand" href="https://openany.si/">OpenAny</a><nav aria-label="`)
	b.WriteString(html.EscapeString(brandGuide))
	b.WriteString(`"><a href="https://openany.si/mcp/">` + html.EscapeString(brandGuide) + `</a><a href="?lang=en" lang="en"`)
	if lang == "en" {
		b.WriteString(` aria-current="page"`)
	}
	b.WriteString(`>English</a><a href="?lang=zh" lang="zh"`)
	if lang == "zh" {
		b.WriteString(` aria-current="page"`)
	}
	b.WriteString(`>中文</a></nav></header>`)
	endpoint := `<div class="endpoint"><span class="endpoint-label">` + html.EscapeString(endpointLabel) + `</span><div class="endpoint-row"><code>https://mcp.openany.si</code><button type="button" class="copy-button" data-copy-endpoint hidden>` + html.EscapeString(copyLabel) + `</button></div><span class="copy-status" role="status" aria-live="polite" data-copy-status></span><div class="endpoint-links"><a href="https://openany.si/mcp/">` + html.EscapeString(brandGuide) + `</a><a href="https://openany.si/dashboard?page=api-keys">` + html.EscapeString(keyLabel) + `</a></div></div>`
	content := renderMarkdown(markdown, lang)
	if end := strings.Index(content, "</h1>"); end >= 0 {
		end += len("</h1>")
		content = content[:end] + endpoint + content[end:]
	}
	b.WriteString(content)
	b.WriteString("</main>" + clipboardScript + "</body></html>")
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
			label := "Reference table"
			if lang == "zh" {
				label = "参考表格"
			}
			b.WriteString(`<div class="table-scroll" role="region" tabindex="0" aria-label="` + label + `"><table><thead><tr>`)
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
			b.WriteString("</tbody></table></div>")
			continue
		}
		if trim == "<!-- developer-docs -->" || trim == "<!-- operations-docs -->" {
			flush()
			if details {
				b.WriteString("</details>")
			}
			summary := "Developer docs"
			if lang == "zh" {
				summary = "开发者文档"
			}
			if trim == "<!-- operations-docs -->" {
				summary = "Deployment reference"
				if lang == "zh" {
					summary = "部署运维参考"
				}
				b.WriteString("<details><summary>")
			} else {
				b.WriteString("<details open><summary>")
			}
			b.WriteString(html.EscapeString(summary))
			b.WriteString("</summary>")
			details = true
			continue
		}
		if trim == "<!-- /operations-docs -->" {
			flush()
			if details {
				b.WriteString("</details>")
				details = false
			}
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
