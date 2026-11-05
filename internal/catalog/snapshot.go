package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/openany/openany-mcp/internal/model"
	"github.com/openany/openany-mcp/internal/schema"
	"github.com/openany/openany-mcp/internal/transport"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Binding struct {
	APIID       string
	ProviderID  string
	Title       string
	Description string
	Input       *schema.Compiled
	Request     model.Request
}
type Issue struct {
	APIID  string `json:"api_id"`
	Reason string `json:"reason"`
}

type Snapshot struct {
	Hash        string
	Tools       map[string]*Binding
	Providers   map[string]bool
	Names       []string
	Active      int
	Pending     int
	Quarantined int
	Issues      []Issue
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var providerPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
var internalDescription = regexp.MustCompile(`(?i)(bearer|api[_ -]?key|authorization|密钥|认证|supplier.cost|internal)`)

func Build(records []model.Record, overrides model.Overrides, checkUnnamed bool) (*Snapshot, error) {
	if overrides.Version != 1 {
		return nil, fmt.Errorf("unsupported overrides version")
	}
	s := &Snapshot{Tools: map[string]*Binding{}, Providers: map[string]bool{}}
	routes := map[string]bool{}
	for _, r := range records {
		if r.Status != "active" {
			continue
		}
		s.Active++
		s.Providers[r.ProviderID] = true
		pending := r.ToolName == nil
		if pending {
			s.Pending++
			if !checkUnnamed {
				continue
			}
		}
		binding, err := compileBinding(r, overrides.APIs[r.APIID])
		if err != nil {
			s.Issues = append(s.Issues, Issue{APIID: r.APIID, Reason: err.Error()})
			if !pending {
				s.Quarantined++
			}
			continue
		}
		if pending {
			continue
		}
		name := *r.ToolName
		if _, ok := s.Tools[name]; ok {
			return nil, fmt.Errorf("duplicate public tool name")
		}
		route := r.ProviderID + ":" + r.Method + ":" + r.PathTemplate
		if routes[route] {
			return nil, fmt.Errorf("duplicate public route")
		}
		routes[route] = true
		s.Tools[name] = binding
		s.Names = append(s.Names, name)
	}
	sort.Strings(s.Names)
	copyRecords := append([]model.Record(nil), records...)
	sort.Slice(copyRecords, func(i, j int) bool { return copyRecords[i].APIID < copyRecords[j].APIID })
	published := map[string]any{}
	for name, binding := range s.Tools {
		published[name] = map[string]any{"inputSchema": binding.Input.Document, "description": binding.Description}
	}
	raw, err := json.Marshal(struct {
		Records   []model.Record
		Overrides model.Overrides
		Published map[string]any
	}{copyRecords, overrides, published})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	s.Hash = hex.EncodeToString(digest[:])
	return s, nil
}

func compileBinding(r model.Record, o model.Override) (*Binding, error) {
	if r.InputError != "" {
		return nil, fmt.Errorf("api %s: %s", r.APIID, r.InputError)
	}
	if r.ToolName != nil && !namePattern.MatchString(*r.ToolName) {
		return nil, fmt.Errorf("api %s: invalid public name", r.APIID)
	}
	if !providerPattern.MatchString(r.ProviderID) || r.PublicPath != "/v1/agent-apis/"+r.ProviderID+r.PathTemplate || !strings.HasPrefix(r.PathTemplate, "/") || strings.HasPrefix(r.PathTemplate, "//") || strings.ContainsAny(r.PublicPath, "?#\\\x00\r\n") {
		return nil, fmt.Errorf("api %s: invalid public route", r.APIID)
	}
	for _, p := range strings.Split(r.PathTemplate, "/") {
		if p == "." || p == ".." {
			return nil, fmt.Errorf("api %s: unsafe static path", r.APIID)
		}
	}
	if (r.Method != "GET" || r.RequestContentType != "none") && (r.Method != "POST" || r.RequestContentType != "application/json") {
		return nil, fmt.Errorf("api %s: unsupported request encoding", r.APIID)
	}
	if !r.ConfigPresent {
		return nil, fmt.Errorf("api %s: missing request config", r.APIID)
	}
	if r.UnknownParameters != "reject" && r.UnknownParameters != "ignore" {
		return nil, fmt.Errorf("api %s: invalid unknown parameter policy", r.APIID)
	}
	if o.ResponseMode != "" && o.ResponseMode != "json" && o.ResponseMode != "unsupported" {
		return nil, fmt.Errorf("api %s: invalid response mode", r.APIID)
	}
	for _, in := range r.Inputs {
		switch in.Location {
		case "path", "query", "body", "header":
		default:
			return nil, fmt.Errorf("api %s: unsupported input location", r.APIID)
		}
		if in.Location == "header" && transport.ReservedHeader(in.Name) {
			return nil, fmt.Errorf("api %s: reserved input header", r.APIID)
		}
		if in.Location != "body" && in.Nullable && (in.Location != "query" || in.Required) {
			return nil, fmt.Errorf("api %s: nullable %s input %s has no supported HTTP encoding; null omission requires an optional query parameter", r.APIID, in.Location, in.Name)
		}
		if in.Type == "array" && in.ItemType == "any" && (len(in.Schema) == 0 || string(in.Schema) == "null") && (in.Location != "body" || r.RequestContentType != "application/json") {
			return nil, fmt.Errorf("api %s: array input %s with item_type any requires a JSON body", r.APIID, in.Name)
		}
		if in.Location == "query" && in.Type == "array" {
			p, ok := o.QueryArrays[in.Name]
			if r.BillingMode == "rules" && p.Encoding == "repeated" {
				return nil, fmt.Errorf("api %s: rules billing cannot accept repeated query keys", r.APIID)
			}
			if !ok || p.Source == "" || (p.Encoding != "csv" && p.Encoding != "repeated" && p.Encoding != "json") || (p.Empty != "omit" && p.Empty != "empty") {
				return nil, fmt.Errorf("api %s: query array %s needs sourced encoding", r.APIID, in.Name)
			}
		}
	}
	request := projectRequest(r, o)
	if err := transport.ValidateInputs(request.Inputs); err != nil {
		return nil, fmt.Errorf("api %s: %w", r.APIID, err)
	}
	compiled, err := schema.Compile(r)
	if err != nil {
		return nil, fmt.Errorf("api %s: input schema invalid: %w", r.APIID, err)
	}
	description := o.Description
	if description == "" {
		description = strings.TrimSpace(strings.Split(r.Description, "\n")[0])
		if internalDescription.MatchString(description) {
			return nil, fmt.Errorf("api %s: public description needs review", r.APIID)
		}
	}
	if description == "" {
		description = r.Name
	}
	if len([]rune(description)) > 600 {
		description = string([]rune(description)[:600])
	}
	description += "\nUses the caller's OpenAny gateway credentials. Charges and authorization are determined by the gateway."
	if o.ResponseMode == "unsupported" {
		description += "\nUnsupported in this version: " + o.UnsupportedReason
	}
	return &Binding{APIID: r.APIID, ProviderID: r.ProviderID, Title: r.Name, Description: description, Input: compiled, Request: request}, nil
}

func projectRequest(r model.Record, o model.Override) model.Request {
	request := model.Request{Method: r.Method, PublicPath: r.PublicPath, RequestContentType: r.RequestContentType, UnsupportedResponse: o.ResponseMode == "unsupported"}
	for _, in := range r.Inputs {
		parameter := model.Parameter{Name: in.Name, Location: in.Location, OmitNull: in.Location == "query" && in.Nullable && !in.Required}
		if in.Pointer != nil {
			pointer := *in.Pointer
			parameter.Pointer = &pointer
		}
		request.Inputs = append(request.Inputs, parameter)
	}
	if len(o.QueryArrays) > 0 {
		request.QueryArrays = make(map[string]model.ArrayPolicy, len(o.QueryArrays))
		for name, policy := range o.QueryArrays {
			request.QueryArrays[name] = model.ArrayPolicy{Encoding: policy.Encoding, Empty: policy.Empty}
		}
	}
	return request
}

type Loader func(context.Context) ([]model.Record, error)
type Manager struct {
	load         Loader
	overrides    model.Overrides
	maxAge       time.Duration
	checkUnnamed bool
	current      atomic.Pointer[Snapshot]
	mu           sync.Mutex
	refreshMu    sync.Mutex
	lastVerified time.Time
	lastErr      bool
	draining     atomic.Bool
}

func NewManager(load Loader, o model.Overrides, maxAge time.Duration, checkUnnamed bool) *Manager {
	return &Manager{load: load, overrides: o, maxAge: maxAge, checkUnnamed: checkUnnamed}
}
func (m *Manager) Refresh(ctx context.Context) error {
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	records, err := m.load(ctx)
	if err == nil {
		var s *Snapshot
		s, err = Build(records, m.overrides, m.checkUnnamed)
		if err == nil {
			old := m.current.Load()
			if old == nil || old.Hash != s.Hash {
				m.current.Store(s)
			}
			m.mu.Lock()
			m.lastVerified = time.Now()
			m.lastErr = false
			m.mu.Unlock()
			return nil
		}
	}
	m.mu.Lock()
	m.lastErr = true
	m.mu.Unlock()
	return err
}
func (m *Manager) Current() *Snapshot { return m.current.Load() }
func (m *Manager) Ready() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.current.Load()
	return !m.draining.Load() && s != nil && (s.Active == 0 || len(s.Tools) > 0) && !m.lastVerified.IsZero() && time.Since(m.lastVerified) < m.maxAge
}
func (m *Manager) Status() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	age := any(nil)
	if !m.lastVerified.IsZero() {
		age = time.Since(m.lastVerified).Seconds()
	}
	s := m.current.Load()
	v := map[string]any{"ready": !m.draining.Load() && s != nil && (s.Active == 0 || len(s.Tools) > 0) && !m.lastVerified.IsZero() && time.Since(m.lastVerified) < m.maxAge, "last_verified_age_seconds": age, "last_refresh_failed": m.lastErr}
	if s != nil {
		v["active"] = s.Active
		v["pending"] = s.Pending
		v["quarantined"] = s.Quarantined
		v["published"] = len(s.Tools)
		v["catalog_hash"] = s.Hash
	}
	return v
}

func (m *Manager) Drain() { m.draining.Store(true) }
