package docs

import (
	_ "embed"
	"strings"
)

//go:embed mcp.md
var MCP string

//go:embed mcp.en.md
var MCPEnglish string

// PlainText keeps all reference content without HTML presentation markers.
func PlainText(markdown string) string {
	for _, marker := range []string{"<!-- developer-docs -->", "<!-- operations-docs -->", "<!-- /operations-docs -->"} {
		markdown = strings.ReplaceAll(markdown, marker+"\n", "")
	}
	return markdown
}
