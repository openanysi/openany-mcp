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
	if strings.Contains(page, "<script>") || strings.Contains(page, "src=") {
		t.Fatal(page)
	}
	for _, body := range []string{MCP, MCPEnglish} {
		rendered := HTML(body, "en")
		if !strings.Contains(rendered, "<h1>") || !strings.Contains(rendered, "<table>") || !strings.Contains(rendered, "<pre><code>") || !strings.Contains(rendered, "https://mcp.openany.si") {
			t.Fatal("guide did not render")
		}
		if strings.Contains(rendered, "<script") || strings.Contains(rendered, "\n# ") {
			t.Fatal("raw markdown or script leaked")
		}
		if !strings.Contains(rendered, "<details><summary>Developer docs</summary>") {
			t.Fatal("developer docs were not collapsed")
		}
		if strings.Index(rendered, "https://mcp.openany.si") > strings.Index(rendered, "<details>") {
			t.Fatal("get started is not above the developer docs")
		}
	}
	zh := HTML(MCP, "zh")
	if !strings.Contains(zh, "lang=\"zh\"") || !strings.Contains(zh, "接入") || !strings.Contains(zh, "<summary>开发者文档</summary>") {
		t.Fatal("chinese guide lost its text")
	}
}
