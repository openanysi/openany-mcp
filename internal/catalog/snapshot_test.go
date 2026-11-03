package catalog

import (
	"context"
	"github.com/openany/openany-mcp/internal/model"
	"testing"
	"time"
)

func fixture(name string) model.Record {
	return model.Record{APIID: "00000000-0000-4000-8000-000000000001", Name: "List", ProviderID: "demo", ToolName: &name, Method: "GET", PathTemplate: "/list", PublicPath: "/v1/agent-apis/demo/list", Status: "active", RequestContentType: "none", UnknownParameters: "reject", ConfigPresent: true}
}
func TestBuildAndAtomicRefresh(t *testing.T) {
	r := fixture("demo_list")
	snap, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Tools) != 1 {
		t.Fatal("missing tool")
	}
	m := NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, false)
	if err = m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	hash := m.Current().Hash
	time.Sleep(time.Millisecond)
	if err = m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.Current().Hash != hash || !m.Ready() {
		t.Fatal("unchanged refresh stale")
	}
	r.Inputs = []model.Input{{Name: "ticker", Type: "array", Location: "query"}}
	if err = m.Refresh(context.Background()); err == nil {
		t.Fatal("accepted missing query encoding")
	}
	if m.Current().Hash != hash {
		t.Fatal("replaced valid snapshot on failure")
	}
}
func TestPendingNamesDoNotPublish(t *testing.T) {
	r := fixture("x")
	r.ToolName = nil
	s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil || s.Pending != 1 || len(s.Tools) != 0 {
		t.Fatalf("pending %+v %v", s, err)
	}
	r.PublicPath = "https://evil.invalid"
	r.ToolName = new(string)
	*r.ToolName = "good"
	if _, err = Build([]model.Record{r}, model.Overrides{Version: 1}, false); err == nil {
		t.Fatal("unsafe route accepted")
	}
}
func TestCatalogExpiresAndRecovers(t *testing.T) {
	r := fixture("demo_list")
	m := NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Second, false)
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.lastVerified = time.Now().Add(-2 * time.Second)
	m.mu.Unlock()
	if m.Ready() {
		t.Fatal("stale directory ready")
	}
	if err := m.Refresh(context.Background()); err != nil || !m.Ready() {
		t.Fatal("did not recover", err)
	}
}
func TestDrainMakesReadinessFalse(t *testing.T) {
	r := fixture("demo_list")
	m := NewManager(func(context.Context) ([]model.Record, error) { return []model.Record{r}, nil }, model.Overrides{Version: 1}, time.Minute, false)
	m.Refresh(context.Background())
	m.Drain()
	if m.Ready() || m.Status()["ready"] != false {
		t.Fatal("draining instance still ready")
	}
}
