package catalog

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openany/openany-mcp/internal/config"
)

// Opt-in compatibility check against an existing catalog. No DDL, fixtures or writes.
func TestReadOnlyPublicMetadataIntegration(t *testing.T) {
	url := os.Getenv("MCP_TEST_READONLY_DATABASE_URL")
	if url == "" {
		t.Skip("set MCP_TEST_READONLY_DATABASE_URL to check existing public metadata read-only")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "15000"
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot initialize read-only pool")
	}
	defer pool.Close()
	var readonly string
	if err := pool.QueryRow(ctx, "SHOW transaction_read_only").Scan(&readonly); err != nil || readonly != "on" {
		t.Fatal("test requires database-enforced read-only mode")
	}
	records, err := PostgresLoader(pool, 15*time.Second)(ctx)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := PostgresMetadataLoader(pool, 15*time.Second)(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prices, err := PostgresPricingLoader(pool, 15*time.Second)(ctx)
	if err != nil {
		t.Fatal(err)
	}
	overrides, err := config.ReadOverrides("../../config/catalog-overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := Build(records, overrides, false)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDiscovery(snapshot, prices, true, metadata)
	samples, valid, translated, categorized := 0, 0, 0, 0
	for _, m := range metadata {
		samples += len(m.Samples)
		if len(m.NameTranslations) > 0 {
			translated++
		}
		if len(m.Categories) > 0 {
			categorized++
		}
	}
	for name := range d.entries {
		detail, _ := d.Detail(name)
		examples := detail["examples"].(map[string]any)
		if examples["status"] == "available" {
			valid++
		}
		if _, err := json.Marshal(detail); err != nil {
			t.Fatal("detail not serializable")
		}
	}
	t.Logf("active=%d direct=%d lite=%d metadata=%d translated=%d categorized=%d source_samples=%d APIs_with_valid_examples=%d; no gateway calls", len(records), len(snapshot.Tools), len(d.entries), len(metadata), translated, categorized, samples, valid)
	if len(metadata) == 0 || len(d.entries) == 0 {
		t.Fatal("empty public metadata/catalog")
	}
}
