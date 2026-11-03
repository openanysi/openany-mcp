package mcpserver

import (
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestPaginationBindsGenerationAndScope(t *testing.T) {
	tools := []*mcp.Tool{}
	for i := 0; i < 51; i++ {
		tools = append(tools, &mcp.Tool{Name: fmt.Sprintf("tool_%02d", i)})
	}
	first, err := page(tools, "hash", "demo", "")
	if err != nil || len(first.Tools) != 50 || first.NextCursor == "" {
		t.Fatal(first, err)
	}
	second, err := page(tools, "hash", "demo", first.NextCursor)
	if err != nil || len(second.Tools) != 1 || second.Tools[0].Name != "tool_50" {
		t.Fatal(second, err)
	}
	if _, err = page(tools, "changed", "demo", first.NextCursor); err == nil {
		t.Fatal("mixed generations")
	}
	if _, err = page(tools, "hash", "other", first.NextCursor); err == nil {
		t.Fatal("mixed scopes")
	}
}
