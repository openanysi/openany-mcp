package config

import (
	"encoding/json"
	"fmt"
	"github.com/openany/openany-mcp/internal/model"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL, GatewayURL, Listen, OverridesFile                                 string
	Hosts, Origins                                                                 []string
	Refresh, MaxAge, QueryTimeout, GatewayTimeout, ConnectTimeout, ShutdownTimeout time.Duration
	MaxConns                                                                       int32
	MaxCalls                                                                       int
	MaxBody, MaxResponse, MaxResult                                                int64
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("POSTGRES_DATABASE_URL"), GatewayURL: os.Getenv("OPENANY_GATEWAY_BASE_URL"), Listen: value("MCP_LISTEN_ADDR", "127.0.0.1:3333"), OverridesFile: value("MCP_CATALOG_OVERRIDES_FILE", "config/catalog-overrides.json"), Hosts: list("MCP_PUBLIC_HOSTS"), Origins: list("MCP_ALLOWED_ORIGINS")}
	if c.DatabaseURL == "" || c.GatewayURL == "" {
		return c, fmt.Errorf("POSTGRES_DATABASE_URL and OPENANY_GATEWAY_BASE_URL are required")
	}
	durations := []struct {
		name, def string
		target    *time.Duration
	}{{"MCP_CATALOG_REFRESH_INTERVAL", "30s", &c.Refresh}, {"MCP_CATALOG_MAX_STALENESS", "120s", &c.MaxAge}, {"MCP_DATABASE_QUERY_TIMEOUT", "10s", &c.QueryTimeout}, {"MCP_GATEWAY_TIMEOUT", "1260s", &c.GatewayTimeout}, {"MCP_GATEWAY_CONNECT_TIMEOUT", "5s", &c.ConnectTimeout}, {"MCP_SHUTDOWN_TIMEOUT", "1300s", &c.ShutdownTimeout}}
	for _, d := range durations {
		n, e := time.ParseDuration(value(d.name, d.def))
		if e != nil || n <= 0 {
			return c, fmt.Errorf("invalid %s", d.name)
		}
		*d.target = n
	}
	for _, n := range []struct {
		name, def string
		target    *int64
	}{{"MCP_REQUEST_MAX_BYTES", "8388608", &c.MaxBody}, {"MCP_GATEWAY_RESPONSE_MAX_BYTES", "8388608", &c.MaxResponse}, {"MCP_RESULT_MAX_BYTES", "20971520", &c.MaxResult}} {
		v, e := strconv.ParseInt(value(n.name, n.def), 10, 64)
		if e != nil || v < 1 || v > 1<<30 {
			return c, fmt.Errorf("invalid %s", n.name)
		}
		*n.target = v
	}
	calls, e := strconv.Atoi(value("MCP_MAX_CONCURRENT_CALLS", "16"))
	if e != nil || calls < 1 || calls > 1024 {
		return c, fmt.Errorf("invalid MCP_MAX_CONCURRENT_CALLS")
	}
	c.MaxCalls = calls
	conns, e := strconv.Atoi(value("MCP_DATABASE_MAX_CONNS", "4"))
	if e != nil || conns < 1 || conns > 100 {
		return c, fmt.Errorf("invalid MCP_DATABASE_MAX_CONNS")
	}
	c.MaxConns = int32(conns)
	if c.MaxAge <= c.Refresh || c.QueryTimeout >= c.MaxAge || c.ConnectTimeout >= c.GatewayTimeout || c.ShutdownTimeout < c.GatewayTimeout {
		return c, fmt.Errorf("inconsistent timeout budgets")
	}
	return c, nil
}
func value(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func list(k string) []string {
	out := []string{}
	for _, s := range strings.Split(os.Getenv(k), ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
func ReadOverrides(path string) (model.Overrides, error) {
	var o model.Overrides
	f, e := os.Open(path)
	if e != nil {
		return o, fmt.Errorf("cannot open catalog overrides")
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(&o); e != nil {
		return o, fmt.Errorf("invalid catalog overrides")
	}
	if o.Version != 1 {
		return o, fmt.Errorf("unsupported overrides version")
	}
	return o, nil
}
