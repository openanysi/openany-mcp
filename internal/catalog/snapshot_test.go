package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/transport"
	"testing"
	"time"
)

func TestRefreshQuarantinesInvalidAPIAndPreservesSnapshotOnDatabaseFailure(t *testing.T) {
	good := fixture("demo_good")
	bad := fixture("demo_bad")
	bad.APIID = "00000000-0000-4000-8000-000000000002"
	bad.PathTemplate, bad.PublicPath = "/other", "/v1/agent-apis/demo/other"
	var loadErr error
	m := NewManager(func(context.Context) ([]model.Record, error) {
		return []model.Record{good, bad}, loadErr
	}, model.Overrides{Version: 1}, time.Minute, false)
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(m.Current().Tools) != 2 {
		t.Fatal("missing initial tools")
	}
	bad.Inputs = []model.Input{{Name: "ticker", Location: "query", Type: "array"}}
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal("one invalid API blocked the healthy catalog:", err)
	}
	if !m.Ready() || len(m.Current().Tools) != 1 || m.Current().Tools["demo_good"] == nil || m.Current().Tools["demo_bad"] != nil {
		t.Fatal("invalid API was not isolated")
	}
	status := m.Status()
	if status["active"] != 2 || status["published"] != 1 || status["quarantined"] != 1 || status["last_refresh_failed"] != false {
		t.Fatalf("wrong partial catalog status: %+v", status)
	}
	hash := m.Current().Hash
	loadErr = errors.New("database unavailable")
	if err := m.Refresh(context.Background()); err == nil || m.Current().Hash != hash || !m.Ready() {
		t.Fatal("database failure replaced the last verified snapshot")
	}
	loadErr = nil
	good.Inputs = bad.Inputs
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.Ready() || m.Status()["ready"] != false || m.Status()["quarantined"] != 2 {
		t.Fatal("catalog with no usable active tools reported ready")
	}
	good.Inputs, bad.Inputs = nil, nil
	if err := m.Refresh(context.Background()); err != nil || !m.Ready() || len(m.Current().Tools) != 2 || m.Status()["quarantined"] != 0 {
		t.Fatal("corrected APIs did not automatically return to the catalog", err)
	}
}

func TestNullableOptionalQueryOmitsOnlyNull(t *testing.T) {
	for _, tc := range []struct {
		name, kind, raw, query string
	}{
		{"null string", "string", `null`, ""},
		{"null integer", "integer", `null`, ""},
		{"null number", "number", `null`, ""},
		{"null boolean", "boolean", `null`, ""},
		{"null array", "array", `null`, ""},
		{"empty string", "string", `""`, "value="},
		{"literal null string", "string", `"null"`, "value=null"},
		{"zero", "integer", `0`, "value=0"},
		{"false", "boolean", `false`, "value=false"},
		{"empty array", "array", `[]`, "value="},
		{"array values", "array", `["a","b"]`, "value=a&value=b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixture("demo_submit")
			r.Method, r.RequestContentType = "POST", "application/json"
			r.Inputs = []model.Input{
				{Name: "value", Location: "query", Type: tc.kind, Nullable: true},
				{Name: "payload", Location: "body", Type: "object", Required: true, Nullable: true},
			}
			o := model.Overrides{Version: 1, APIs: map[string]model.Override{r.APIID: {QueryArrays: map[string]model.ArrayEncoding{
				"value": {Encoding: "repeated", Empty: "empty", Source: "test contract"},
			}}}}
			snap, err := Build([]model.Record{r}, o, false)
			if err != nil {
				t.Fatal(err)
			}
			b := snap.Tools["demo_submit"]
			args, err := b.Input.Validate([]byte(`{"value":` + tc.raw + `,"payload":null}`))
			if err != nil {
				t.Fatal(err)
			}
			e, err := transport.Encode(b.Request, args)
			if err != nil {
				t.Fatal(err)
			}
			if e.Query != tc.query || string(e.Body) != `{"payload":null}` {
				t.Fatalf("query=%q body=%s", e.Query, e.Body)
			}
			if _, err := b.Input.Validate(json.RawMessage(`{"payload":null}`)); err != nil {
				t.Fatal("optional query became required:", err)
			}
		})
	}
}

func TestNullableHTTPInputsStillRequireSupportedPolicy(t *testing.T) {
	for _, tc := range []struct {
		location string
		required bool
	}{{"path", true}, {"header", false}, {"query", true}} {
		r := fixture("demo_list")
		r.Inputs = []model.Input{{Name: "value", Location: tc.location, Type: "string", Nullable: true, Required: tc.required}}
		s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
		if err != nil || len(s.Tools) != 0 || s.Quarantined != 1 {
			t.Fatalf("unsupported nullable input was not isolated: %+v %v", tc, err)
		}
	}
}

func fixture(name string) model.Record {
	return model.Record{APIID: "00000000-0000-4000-8000-000000000001", Name: "List", ProviderID: "demo", ToolName: &name, Method: "GET", PathTemplate: "/list", PublicPath: "/v1/agent-apis/demo/list", Status: "active", RequestContentType: "none", UnknownParameters: "reject", ConfigPresent: true}
}

func TestAnyArrayToolPublishesAndPreservesBodyPlacement(t *testing.T) {
	for _, tc := range []struct{ pointer, body string }{
		{"", `["link",9007199254740993,false,null,{},[1,"nested"]]`},
		{"/links", `{"links":["link",9007199254740993,false,null,{},[1,"nested"]]}`},
	} {
		r := fixture("demo_batch_post_ids")
		r.APIID = "0007569f-ced2-4463-a3e6-b32da069a2f3"
		r.Name = "Get post IDs in batch through sharing links"
		r.Method, r.RequestContentType = "POST", "application/json"
		pointer := tc.pointer
		r.Inputs = []model.Input{{Name: "body", Location: "body", Type: "array", ItemType: "any", Required: true, Pointer: &pointer, Schema: json.RawMessage(`null`)}}
		s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
		if err != nil || s.Quarantined != 0 || len(s.Tools) != 1 {
			t.Fatalf("valid any-array API was not published: %v", err)
		}
		b := s.Tools["demo_batch_post_ids"]
		args, err := b.Input.Validate([]byte(`{"body":["link",9007199254740993,false,null,{},[1,"nested"]]}`))
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := transport.Encode(b.Request, args)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded.Body) != tc.body {
			t.Fatalf("body placement or JSON values changed: got %s, want %s", encoded.Body, tc.body)
		}
	}
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
	records := []model.Record{r}
	m := NewManager(func(context.Context) ([]model.Record, error) { return records, nil }, model.Overrides{Version: 1}, time.Minute, false)
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
	records = append(records, r)
	if err = m.Refresh(context.Background()); err == nil {
		t.Fatal("accepted duplicate tool identity")
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
	s, err = Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil || len(s.Tools) != 0 || s.Quarantined != 1 {
		t.Fatal("unsafe route was not isolated", err)
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
func TestCatalogRejectsOverlappingDecodedBodyPointers(t *testing.T) {
	r := fixture("demo_list")
	r.Method = "POST"
	r.RequestContentType = "application/json"
	a := "/a~1b"
	b := "/a~1b/c"
	r.Inputs = []model.Input{{Name: "parent", Type: "object", Location: "body", Pointer: &a}, {Name: "child", Type: "integer", Location: "body", Pointer: &b}}
	s, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil || len(s.Tools) != 0 || s.Quarantined != 1 {
		t.Fatal("catalog failed to isolate overlapping decoded pointers", err)
	}
}
func TestSlowRefreshDoesNotBlockCurrentReadiness(t *testing.T) {
	r := fixture("demo_list")
	blocked := make(chan struct{})
	release := make(chan struct{})
	calls := 0
	m := NewManager(func(context.Context) ([]model.Record, error) {
		calls++
		if calls == 2 {
			close(blocked)
			<-release
		}
		return []model.Record{r}, nil
	}, model.Overrides{Version: 1}, time.Minute, false)
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { m.Refresh(context.Background()); close(done) }()
	<-blocked
	checked := make(chan bool, 1)
	go func() { checked <- m.Ready() }()
	select {
	case ready := <-checked:
		if !ready {
			t.Error("valid previous snapshot unavailable")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("database refresh blocked current snapshot readiness")
	}
	close(release)
	<-done
}

func TestRefreshOnlyRevisionStillInvalidatesCatalogHash(t *testing.T) {
	r := fixture("demo_list")
	before, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	r.Revision++
	after, err := Build([]model.Record{r}, model.Overrides{Version: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	if before.Hash == after.Hash {
		t.Fatal("discarded refresh-only metadata stopped invalidating catalog cursors")
	}
}
