package main

import (
	"encoding/json"
	"io"
	"log/slog"

	"github.com/openany/openany-mcp/internal/catalog"
)

func writeCatalogReport(out io.Writer, manager *catalog.Manager) int {
	status := manager.Status()
	snapshot := manager.Current()
	status["issues"] = snapshot.Issues
	status["warnings"] = snapshot.Warnings
	if err := json.NewEncoder(out).Encode(status); err != nil || len(snapshot.Issues) > 0 {
		return 1
	}
	return 0
}

func logCatalogDiagnostics(logger *slog.Logger, snapshot *catalog.Snapshot, previousHash string) string {
	if snapshot.Hash == previousHash {
		return previousHash
	}
	identity := func(issue catalog.Issue) []any {
		return []any{"api_id", issue.APIID, "provider_id", issue.ProviderID, "method", issue.Method, "path_template", issue.PathTemplate, "request_content_type", issue.RequestContentType}
	}
	for _, issue := range snapshot.Issues {
		// Detailed schema errors belong in --check-catalog; source values may be private.
		logger.Warn("catalog API quarantined", identity(issue)...)
	}
	for _, warning := range snapshot.Warnings {
		logger.Warn("catalog API warning", append(identity(warning), "reason", warning.Reason)...)
	}
	return snapshot.Hash
}
