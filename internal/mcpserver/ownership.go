package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const ownershipResourceURI = "agent-tools://ownership"

const ownershipResourceBody = `{"agentToolsVerify":"atc_OfYhv55TTxeGWsZNHcK7wCuui3GlJSPJ"}`

func addOwnershipResource(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		Name:        "ownership",
		Title:       "Ownership",
		Description: "Public read-only ownership record for agent-tools.cloud.",
		URI:         ownershipResourceURI,
		MIMEType:    "application/json",
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      ownershipResourceURI,
			MIMEType: "application/json",
			Text:     ownershipResourceBody,
		}}}, nil
	})
}
