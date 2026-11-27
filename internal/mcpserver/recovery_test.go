package mcpserver

import (
	"github.com/openany/openany-mcp/internal/gateway"
	"strings"
	"testing"
)

func TestRecoveryRenderingPreservesValidationAndUnknownOutcome(t *testing.T) {
	r := gateway.Failure("openany_insufficient_balance", "balance insufficient", "gateway", "not_started", 402, "req")
	e := r.Value["error"].(map[string]any)
	e["recovery"] = map[string]any{"primary_action": "top_up", "next_step_target": "https://openany.si/dashboard?page=credits"}
	e["details"] = []any{map[string]any{"path": "/q"}}
	e["details_scope"] = "input"
	r = withRecovery(r)
	if !strings.Contains(r.Value["error"].(map[string]any)["next_action"].(string), "credits") || e["details_scope"] != "input" {
		t.Fatal("recovery/validation lost", r)
	}
	e["execution_state"] = "unknown"
	withRecovery(r)
	if !strings.Contains(e["next_action"].(string), "Do not retry") {
		t.Fatal("unknown state was overridden", r)
	}
}
