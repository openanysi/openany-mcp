package config

import (
	"os"
	"strings"
	"testing"
)

func TestLiteFlagDefaultsOnAndValidatesValues(t *testing.T) {
	t.Setenv("POSTGRES_DATABASE_URL", "postgres://localhost/test")
	t.Setenv("OPENANY_GATEWAY_BASE_URL", "http://localhost:8888")
	t.Setenv("MCP_LITE_ENABLED", "")
	c, err := Load()
	if err != nil || !c.LiteEnabled {
		t.Fatal("default disabled or invalid", err)
	}
	t.Setenv("MCP_LITE_ENABLED", "false")
	c, err = Load()
	if err != nil || c.LiteEnabled {
		t.Fatal("explicit disable ignored", err)
	}
	t.Setenv("MCP_LITE_ENABLED", "maybe")
	if _, err = Load(); err == nil {
		t.Fatal("invalid boolean accepted")
	}
	t.Setenv("MCP_LITE_ENABLED", "true")
	t.Setenv("OPENANY_MODELS_BASE_URL", "https://api.openany.si/v1")
	c, err = Load()
	if err != nil || c.ModelsOrigin != "https://api.openany.si" {
		t.Fatal(c.ModelsOrigin, err)
	}
	t.Setenv("OPENANY_MODELS_BASE_URL", "https://api.openany.si/v1/models")
	if _, err = Load(); err == nil {
		t.Fatal("model catalog path accepted")
	}
}

func TestAccountConfigurationFlagsAndOrigin(t *testing.T) {
	t.Setenv("POSTGRES_DATABASE_URL", "postgres://localhost/test")
	t.Setenv("OPENANY_GATEWAY_BASE_URL", "http://localhost:8888")
	for _, name := range []string{"MCP_ACCOUNT_ENABLED", "MCP_BENEFITS_ENABLED", "MCP_ACCOUNT_BASE_URL"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	c, err := Load()
	if err != nil || c.AccountEnabled || c.BenefitsEnabled || c.AccountOrigin != "https://openany.si" {
		t.Fatal("unexpected account defaults", c.AccountEnabled, c.BenefitsEnabled, c.AccountOrigin, err)
	}
	t.Setenv("MCP_ACCOUNT_ENABLED", "true")
	t.Setenv("MCP_BENEFITS_ENABLED", "false")
	t.Setenv("MCP_ACCOUNT_BASE_URL", "https://account.example.org/")
	c, err = Load()
	if err != nil || !c.AccountEnabled || c.BenefitsEnabled || c.AccountOrigin != "https://account.example.org" {
		t.Fatal("account controls not independent", c.AccountEnabled, c.BenefitsEnabled, c.AccountOrigin, err)
	}
	for _, flag := range []string{"MCP_ACCOUNT_ENABLED", "MCP_BENEFITS_ENABLED"} {
		for _, bad := range []string{"", " ", "1", "maybe"} {
			t.Setenv(flag, bad)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), flag) {
				t.Errorf("invalid %s accepted", flag)
			}
		}
		t.Setenv(flag, "false")
	}
	t.Setenv("MCP_ACCOUNT_BASE_URL", "https://account.example.org/api")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MCP_ACCOUNT_BASE_URL") {
		t.Fatal("account origin accepted a path")
	}
}
