package config

import "testing"

func TestLiteFlagDefaultsOffAndValidatesValues(t *testing.T) {
	t.Setenv("POSTGRES_DATABASE_URL", "postgres://localhost/test")
	t.Setenv("OPENANY_GATEWAY_BASE_URL", "http://localhost:8888")
	t.Setenv("MCP_LITE_ENABLED", "")
	c, err := Load()
	if err != nil || c.LiteEnabled {
		t.Fatal("default enabled or invalid", err)
	}
	t.Setenv("MCP_LITE_ENABLED", "true")
	c, err = Load()
	if err != nil || !c.LiteEnabled {
		t.Fatal("flag ignored", err)
	}
	t.Setenv("MCP_LITE_ENABLED", "maybe")
	if _, err = Load(); err == nil {
		t.Fatal("invalid boolean accepted")
	}
}
