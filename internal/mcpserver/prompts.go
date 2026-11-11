package mcpserver

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const startPromptEN = "What can OpenAny do for me? Please call openany_overview first, introduce it, and give me example questions I can ask directly."

const startPromptZH = "OpenAny 能帮我做什么？请先调用 openany_overview 介绍一下，并给我一些可以直接问的示例。"

func addStartPrompt(s *mcp.Server) {
	s.AddPrompt(&mcp.Prompt{
		Name:        "openany-start",
		Title:       "开始使用 OpenAny / Get started with OpenAny",
		Description: "Introduce OpenAny and suggest questions the user can ask.",
		Arguments: []*mcp.PromptArgument{{
			Name:        "lang",
			Title:       "Language",
			Description: "zh or en. Defaults to en.",
		}},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		lang := "en"
		if req != nil && req.Params != nil {
			if value := strings.ToLower(strings.TrimSpace(req.Params.Arguments["lang"])); value == "zh" || value == "en" {
				lang = value
			}
		}
		text := startPromptEN
		if lang == "zh" {
			text = startPromptZH
		}
		return &mcp.GetPromptResult{
			Description: "Ask the assistant to introduce OpenAny from the live catalog.",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: text},
			}},
		}, nil
	})
}
