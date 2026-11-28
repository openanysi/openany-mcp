package docs

import (
	"strings"
	"testing"
)

func TestGuideHTMLRendersStructureAndEscapes(t *testing.T) {
	page := HTML("# Title\n\nSee `code` and <script>alert(1)</script>.\n\n| A | B |\n| --- | --- |\n| 1 | 2 |\n\n```json\n{\"n\":1}\n```\n", "en")
	for _, want := range []string{"<h1>Title</h1>", "<table>", "<th>A</th>", "<td>1</td>", "<pre><code>", "<code>code</code>", "&lt;script&gt;"} {
		if !strings.Contains(page, want) {
			t.Fatalf("missing %s\n%s", want, page)
		}
	}
	if strings.Contains(strings.Replace(page, clipboardScript, "", 1), "<script") || strings.Contains(page, "src=") || strings.Count(page, clipboardScript) != 1 {
		t.Fatal(page)
	}
	for _, body := range []string{MCP, MCPEnglish} {
		rendered := HTML(body, "en")
		if !strings.Contains(rendered, "<h1>") || !strings.Contains(rendered, "<table>") || !strings.Contains(rendered, "<pre><code>") || !strings.Contains(rendered, "https://mcp.openany.si") {
			t.Fatal("guide did not render")
		}
		if strings.Contains(strings.Replace(rendered, clipboardScript, "", 1), "<script") || strings.Contains(rendered, "\n# ") {
			t.Fatal("raw markdown or script leaked")
		}
		if !strings.Contains(rendered, "<details open><summary>Developer docs</summary>") {
			t.Fatal("developer docs were not expanded")
		}
		if strings.Index(rendered, "https://mcp.openany.si") > strings.Index(rendered, "<details open>") {
			t.Fatal("get started is not above the developer docs")
		}
	}
	zh := HTML(MCP, "zh")
	if !strings.Contains(zh, "lang=\"zh\"") || !strings.Contains(zh, "接入") || !strings.Contains(zh, "<summary>开发者文档</summary>") {
		t.Fatal("chinese guide lost its text")
	}
}

func TestGuideLinksCopyAndDisclosureBoundaries(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		body := MCPEnglish
		if lang == "zh" {
			body = MCP
		}
		page := HTML(body, lang)
		for _, want := range []string{`class="table-scroll" role="region" tabindex="0"`, `data-copy-endpoint hidden`, `role="status" aria-live="polite"`, `href="https://openany.si/mcp/"`, `href="https://openany.si/dashboard?page=api-keys"`, `href="https://openany.si/ai-info/"`, `href="https://openany.si/llms.txt"`, `href="https://openany.si/AUTH.md"`, `href="?lang=en"`, `href="?lang=zh"`} {
			if !strings.Contains(page, want) {
				t.Fatalf("%s missing %s", lang, want)
			}
		}
		if strings.Count(page, "<details open>") != 1 || strings.Count(page, "<details><summary>") != 1 || strings.Count(page, "</details>") != 2 {
			t.Fatal("reference disclosure structure changed", lang)
		}
		operations := strings.Index(page, "<details><summary>")
		operationsEnd := strings.Index(page[operations:], "</details>") + operations
		if !(operations < strings.Index(page, "MCP_LITE_ENABLED=false") && strings.Index(page, "MCP_PUBLIC_HOSTS") < operationsEnd) {
			t.Fatal("operations content left its disclosure", lang)
		}
		if strings.Index(page, `href="https://openany.si/ai-info/"`) < operationsEnd {
			t.Fatal("public references are hidden inside operations", lang)
		}
		plain := PlainText(body)
		if strings.Contains(plain, "<!--") || strings.Contains(plain, "<script") || !strings.Contains(plain, "MCP_PUBLIC_HOSTS") {
			t.Fatal("plain text lost content or leaked presentation", lang)
		}
		for _, stale := range []string{"168+", "4,400+", "3,200+", "any MCP client", "任意 MCP 客户端", "and you pay per call"} {
			if strings.Contains(body, stale) {
				t.Fatal("stale or overbroad claim", lang, stale)
			}
		}
	}
	if !strings.Contains(clipboardScript, `writeText('https://mcp.openany.si')`) || strings.Contains(clipboardScript, "fetch(") {
		t.Fatal("copy must only write the public endpoint, without network calls")
	}
}

func TestGuideRejectsExecutableMarkdown(t *testing.T) {
	page := HTML("# Safe\n\n[bad](javascript:alert) <img src=x onerror=alert(1)> [good](https://openany.si/mcp/)\n", "en")
	page = strings.Replace(page, clipboardScript, "", 1)
	for _, unsafe := range []string{"<script", "<img", `href="javascript:`} {
		if strings.Contains(page, unsafe) {
			t.Fatal("executable markup escaped the renderer", unsafe)
		}
	}
	if !strings.Contains(page, `href="https://openany.si/mcp/"`) || !strings.Contains(page, "&lt;img") {
		t.Fatal("safe links or escaped text lost")
	}
}

func TestGuideDescribesCatalogCategoriesInBothFormats(t *testing.T) {
	for _, doc := range []struct{ lang, body, apis, models, scope string }{
		{"en", MCPEnglish, "API categories", "Model categories", "Execution scope"},
		{"zh", MCP, "API 类别", "模型类别", "执行范围"},
	} {
		for _, text := range []string{PlainText(doc.body), HTML(doc.body, doc.lang)} {
			for _, want := range []string{doc.apis, doc.models, doc.scope, "DataForSEO", "TikHub", "CoinGecko", "Hunter", "FRED", "PKULaw", "Open-Meteo", "DeepSeek", "FLUX", "Kling", "music-v2", "get_api"} {
				if !strings.Contains(text, want) {
					t.Fatal(doc.lang, "missing category or example", want)
				}
			}
			if !(strings.Index(text, doc.apis) < strings.Index(text, doc.models) && strings.Index(text, doc.models) < strings.Index(text, doc.scope)) {
				t.Fatal(doc.lang, "catalog examples must precede execution boundaries")
			}
		}
	}
}
