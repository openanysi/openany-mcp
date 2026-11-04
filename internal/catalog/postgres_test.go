package catalog

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openany/openany-mcp/internal/model"
)

func TestPostgresMalformedInputsDoNotBlockOtherAPIs(t *testing.T) {
	databaseURL := os.Getenv("MCP_TEST_CATALOG_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set MCP_TEST_CATALOG_DATABASE_URL to a disposable openany_mcp_test database")
	}
	pc, err := pgxpool.ParseConfig(databaseURL)
	if err != nil || !strings.HasPrefix(pc.ConnConfig.Database, "openany_mcp_test_") {
		t.Fatal("refusing to recreate catalog tables outside an openany_mcp_test_ database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `
DROP TABLE IF EXISTS public.agent_api_request_configs, public.agent_apis;
CREATE TABLE public.agent_apis (
 id uuid PRIMARY KEY, name text, description text, provider_id text, public_mcp_tool_name text,
 method text, path_template text, public_path text, status text, revision integer,
 upstream_protocol text, request_content_type text, unknown_parameters text, billing_mode text
);
CREATE TABLE public.agent_api_request_configs (api_id uuid PRIMARY KEY, inputs jsonb CHECK (jsonb_typeof(inputs)='array'));
INSERT INTO public.agent_apis
 SELECT ('00000000-0000-4000-8000-00000000000'||n)::uuid, 'List', '', 'demo',
 CASE WHEN n=3 THEN NULL ELSE 'demo_list_'||n END,
 'GET', '/list/'||n, '/v1/agent-apis/demo/list/'||n, 'active', 1, 'http', 'none', 'reject', 'fixed'
 FROM generate_series(1,3) n;
INSERT INTO public.agent_api_request_configs
 SELECT id, CASE WHEN public_mcp_tool_name='demo_list_1' THEN '[]'::jsonb
 ELSE '[{"name":"q","location":"query","type":"string","required":"false"}]'::jsonb END
 FROM public.agent_apis;`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP TABLE public.agent_api_request_configs, public.agent_apis"); err != nil {
			t.Error(err)
		}
	})
	pc, _ = pgxpool.ParseConfig(databaseURL)
	pc.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	reader, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	records, err := PostgresLoader(reader, 5*time.Second)(ctx)
	if err != nil {
		t.Fatal("one malformed API blocked the database loader:", err)
	}
	for _, checkUnnamed := range []bool{false, true} {
		s, err := Build(records, model.Overrides{Version: 1}, checkUnnamed)
		if err != nil || len(s.Tools) != 1 || s.Tools["demo_list_1"] == nil || s.Quarantined != 1 || s.Pending != 1 {
			t.Fatalf("malformed metadata was not isolated (checkUnnamed=%v): %v", checkUnnamed, err)
		}
		wantIssues := 1
		if checkUnnamed {
			wantIssues = 2
		}
		if len(s.Issues) != wantIssues {
			t.Fatalf("issues=%d, want %d", len(s.Issues), wantIssues)
		}
	}
}
