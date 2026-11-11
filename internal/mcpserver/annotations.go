package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

func boolHint(v bool) *bool { return &v }

func readOnlyAnnotations(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    true,
		DestructiveHint: boolHint(false),
		IdempotentHint:  true,
		OpenWorldHint:   boolHint(false),
	}
}

func executionAnnotations(title string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    false,
		DestructiveHint: boolHint(false),
		IdempotentHint:  false,
		OpenWorldHint:   boolHint(true),
	}
}
