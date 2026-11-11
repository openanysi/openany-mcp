package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/openany/openany-mcp/internal/schema"
	js "github.com/santhosh-tekuri/jsonschema/v6"
)

// LLMModel is one row from the OpenAI-compatible GET /v1/models catalog.
// Fields stay empty when the catalog does not provide them.
type LLMModel struct {
	ID               string
	Name             string
	Provider         string
	InputModalities  []string
	OutputModalities []string
	Features         []string
	Sampling         []string
	ContextLength    int
	MaxOutput        int
	Pricing          json.RawMessage
	OpenRouterSlug   string
	HuggingFace      string
	Free             *bool
}

type ModelLoader func(context.Context) ([]LLMModel, error)

type modelEntry struct {
	model        LLMModel
	fields       [6]string
	summaryText  string
	pricingBrief string
	version      string
	limitations  []string
}

const pricingNote = "Catalog token prices are USD per token. pricing_brief multiplies prompt, completion and input_cache_read by 1000000. image, request and duration amounts are shown as published. This is not a price lock."

const chatSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": true,
  "required": ["messages"],
  "properties": {
    "model": {"type": "string", "minLength": 1, "description": "Optional. When set, it must equal this model's model_id. When omitted, the server sends model_id."},
    "stream": {"type": "boolean", "description": "true is rejected. This tool returns one JSON body."},
    "messages": {
      "type": "array",
      "minItems": 1,
      "description": "Chat completions messages. Each item needs role and content.",
      "items": {
        "type": "object",
        "additionalProperties": true,
        "required": ["role", "content"],
        "properties": {
          "role": {"type": "string", "minLength": 1},
          "content": {"description": "Text or the content parts accepted by this model."}
        }
      }
    }
  }
}`

var chatInput *schema.Compiled

func init() {
	var document map[string]any
	if err := json.Unmarshal([]byte(chatSchemaJSON), &document); err != nil {
		panic(err)
	}
	compiled, err := compileDocument(document)
	if err != nil {
		panic(err)
	}
	chatInput = compiled
}

func compileDocument(document map[string]any) (*schema.Compiled, error) {
	compiler := js.NewCompiler()
	compiler.DefaultDraft(js.Draft2020)
	compiler.UseLoader(nil)
	if err := compiler.AddResource("urn:openany:model-input", document); err != nil {
		return nil, err
	}
	validator, err := compiler.Compile("urn:openany:model-input")
	if err != nil {
		return nil, err
	}
	return &schema.Compiled{Document: document, Validator: validator}, nil
}

func ChatInputSchema() *schema.Compiled { return chatInput }

// Search synonyms for an id prefix. They are not catalog fields and are not shown as the provider.
var modelVendorAliases = map[string]string{
	"claude":   "anthropic",
	"gpt":      "openai",
	"gptimage": "openai",
	"qwen":     "alibaba 通义千问",
	"gemini":   "google",
	"llama":    "meta",
	"kimi":     "moonshot",
	"glm":      "zai",
	"grok":     "xai",
}

var modelHTTP = &http.Client{
	Transport: &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func HTTPModelLoader(origin string, timeout time.Duration) ModelLoader {
	return func(ctx context.Context) ([]LLMModel, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(origin, "/")+"/v1/models", nil)
		if err != nil {
			return nil, fmt.Errorf("model catalog request failed")
		}
		req.Header.Set("Accept", "application/json")
		resp, err := modelHTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("model catalog request failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("model catalog request failed")
		}
		media := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
		if media != "application/json" && !strings.HasSuffix(media, "+json") {
			return nil, fmt.Errorf("model catalog request failed")
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20+1))
		if err != nil || len(raw) > 8<<20 {
			return nil, fmt.Errorf("model catalog request failed")
		}
		return ParseModelCatalog(raw)
	}
}

func ParseModelCatalog(raw []byte) ([]LLMModel, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var body any
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("model catalog decode failed")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("model catalog decode failed")
	}
	var items []any
	switch v := body.(type) {
	case []any:
		items = v
	case map[string]any:
		arr, ok := v["data"].([]any)
		if !ok {
			return nil, fmt.Errorf("model catalog decode failed")
		}
		items = arr
	default:
		return nil, fmt.Errorf("model catalog decode failed")
	}
	models := []LLMModel{}
	seen := map[string]bool{}
	skipped := 0
	for _, item := range items {
		model, ok := parseModelItem(item)
		if !ok {
			skipped++
			continue
		}
		if seen[model.ID] {
			continue
		}
		seen[model.ID] = true
		models = append(models, model)
	}
	if len(items) > 0 && len(models) == 0 && skipped == len(items) {
		return nil, fmt.Errorf("model catalog decode failed")
	}
	return models, nil
}

func parseModelItem(item any) (LLMModel, bool) {
	raw, ok := item.(map[string]any)
	if !ok {
		return LLMModel{}, false
	}
	id, _ := raw["id"].(string)
	id = strings.TrimSpace(id)
	if id == "" || utf8.RuneCountInString(id) > 64 || strings.ContainsAny(id, "\r\n\x00") {
		return LLMModel{}, false
	}
	if ready, exists := raw["is_ready"].(bool); exists && !ready {
		return LLMModel{}, false
	}
	name, _ := raw["name"].(string)
	model := LLMModel{
		ID:               id,
		Name:             strings.TrimSpace(name),
		Provider:         providerToken(id),
		InputModalities:  stringList(raw["input_modalities"]),
		OutputModalities: stringList(raw["output_modalities"]),
		Features:         stringList(raw["supported_features"]),
		Sampling:         stringList(raw["supported_sampling_parameters"]),
		ContextLength:    positiveInt(raw["context_length"]),
		MaxOutput:        positiveInt(raw["max_output_length"]),
		HuggingFace:      strings.TrimSpace(asString(raw["hugging_face_id"])),
	}
	if raw["pricing"] != nil {
		encoded, err := json.Marshal(raw["pricing"])
		if err == nil && string(encoded) != "null" {
			model.Pricing = encoded
		}
	}
	if router, exists := raw["openrouter"].(map[string]any); exists {
		model.OpenRouterSlug = strings.TrimSpace(asString(router["slug"]))
	}
	if free, exists := raw["is_free"].(bool); exists {
		model.Free = &free
	}
	return model, true
}

func providerToken(id string) string {
	lower := strings.ToLower(id)
	i := 0
	for i < len(lower) {
		c := lower[i]
		if c < 'a' || c > 'z' {
			break
		}
		i++
	}
	if i == 0 {
		return "openany"
	}
	if i > 64 {
		i = 64
	}
	return lower[:i]
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func stringList(v any) []string {
	items, _ := v.([]any)
	out := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		s := strings.TrimSpace(asString(item))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func positiveInt(v any) int {
	number, ok := v.(json.Number)
	if !ok {
		return 0
	}
	n, err := number.Int64()
	if err != nil || n <= 0 || n > 1_000_000_000 {
		return 0
	}
	return int(n)
}

func (m LLMModel) displayName() string {
	if strings.TrimSpace(m.Name) != "" {
		return strings.TrimSpace(m.Name)
	}
	return m.ID
}

func (m LLMModel) modalityList() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range append(append([]string{}, m.InputModalities...), m.OutputModalities...) {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func (m LLMModel) hasModality(name string) bool {
	return hasToken(m.InputModalities, name) || hasToken(m.OutputModalities, name)
}

func hasToken(values []string, name string) bool {
	for _, item := range values {
		if strings.EqualFold(item, name) {
			return true
		}
	}
	return false
}

func (m LLMModel) hasInput(name string) bool  { return hasToken(m.InputModalities, name) }
func (m LLMModel) hasOutput(name string) bool { return hasToken(m.OutputModalities, name) }

// textOutput is a catalog output of chat, coding, or reasoning.
// Input "chat" is only the prompt channel and is also set on image and video models.
func (m LLMModel) textOutput() bool {
	return m.hasOutput("chat") || m.hasOutput("coding") || m.hasOutput("reasoning")
}

func (m LLMModel) hasFeature(name string) bool {
	for _, item := range m.Features {
		if strings.EqualFold(item, name) {
			return true
		}
	}
	return false
}

func (m LLMModel) matchesProvider(want string) bool {
	if strings.EqualFold(want, m.Provider) {
		return true
	}
	alias := lexical(modelVendorAliases[m.Provider])
	return alias != "" && containsWord(alias, lexical(want))
}

func (m LLMModel) allowsCapability(want string) bool {
	for _, item := range m.modalityList() {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	for _, item := range m.Features {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}

func (m LLMModel) matchesCategory(want string) bool {
	switch strings.ToLower(strings.TrimSpace(want)) {
	case "models", "llm", "ai-generation":
		return true
	default:
		return m.allowsCapability(want)
	}
}

func (m LLMModel) taskText() string {
	parts := []string{"model", "llm"}
	id := lexical(m.ID)
	if m.textOutput() {
		parts = append(parts, "chat", "completion", "writing", "translation")
		if m.hasInput("coding") || m.hasOutput("coding") || containsWord(id, "code") {
			parts = append(parts, "code", "coding")
		}
		// vision feature counts as image input. This catalog often records it there instead of input_modalities.
		if m.hasInput("image") || m.hasFeature("vision") {
			parts = append(parts, "vision")
		}
	}
	if m.hasOutput("image") || m.hasFeature("image_generation") {
		parts = append(parts, "imagegen")
	}
	if m.hasModality("audio") || m.hasFeature("audio_input") || m.hasFeature("audio_output") {
		parts = append(parts, "audio")
	}
	if m.hasModality("video") {
		parts = append(parts, "video")
	}
	if m.hasFeature("embedding") || containsWord(id, "embedding") {
		parts = append(parts, "embedding")
	}
	return strings.Join(parts, " ")
}

func (m LLMModel) publicSummary() string {
	parts := []string{}
	if mods := m.modalityList(); len(mods) > 0 {
		parts = append(parts, "Modalities: "+strings.Join(mods, ", "))
	}
	if m.ContextLength > 0 {
		parts = append(parts, fmt.Sprintf("Context length: %d", m.ContextLength))
	}
	if len(m.Features) > 0 {
		parts = append(parts, "Features: "+strings.Join(m.Features, ", "))
	}
	if m.Free != nil && *m.Free {
		parts = append(parts, "Catalog marks this model free")
	}
	if len(parts) == 0 {
		return m.displayName()
	}
	return short(strings.Join(parts, ". "), 160)
}

// searchableModalities is the modality text used for keyword search.
// Input "chat" is only the prompt channel. Image and video models list it too,
// so it is not a chat-task hit unless the model actually emits text.
func (m LLMModel) searchableModalities() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(item string) {
		item = strings.TrimSpace(item)
		if item == "" || seen[strings.ToLower(item)] {
			return
		}
		if strings.EqualFold(item, "chat") && !m.textOutput() {
			return
		}
		seen[strings.ToLower(item)] = true
		out = append(out, item)
	}
	for _, item := range m.OutputModalities {
		add(item)
	}
	for _, item := range m.InputModalities {
		add(item)
	}
	sort.Strings(out)
	return out
}

func (m LLMModel) searchSummary() string {
	parts := []string{}
	if mods := m.searchableModalities(); len(mods) > 0 {
		parts = append(parts, "Modalities: "+strings.Join(mods, ", "))
	}
	if m.ContextLength > 0 {
		parts = append(parts, fmt.Sprintf("Context length: %d", m.ContextLength))
	}
	if len(m.Features) > 0 {
		parts = append(parts, "Features: "+strings.Join(m.Features, ", "))
	}
	if m.Free != nil && *m.Free {
		parts = append(parts, "Catalog marks this model free")
	}
	if len(parts) == 0 {
		return m.displayName()
	}
	return short(strings.Join(parts, ". "), 160)
}

// primaryModality is the single overview group for this model.
// Text output stays with chat/reasoning, including vision LLMs.
// Image, video, and audio are models that do not emit text.
func (m LLMModel) primaryModality() string {
	if m.textOutput() {
		return "chat/reasoning"
	}
	if m.hasOutput("video") {
		return "video"
	}
	if m.hasOutput("image") || m.hasFeature("image_generation") {
		return "image"
	}
	if m.hasModality("audio") || m.hasFeature("audio_input") || m.hasFeature("audio_output") {
		return "audio"
	}
	if m.hasModality("video") {
		return "video"
	}
	return "other"
}

func (m LLMModel) searchFields() [6]string {
	vendor := modelVendorAliases[m.Provider]
	extra := vendor
	if m.OpenRouterSlug != "" {
		extra += " openrouter"
	}
	if m.HuggingFace != "" {
		extra += " " + m.HuggingFace
	}
	class := strings.Join([]string{"models", "llm", "ai-generation", strings.Join(m.searchableModalities(), " "), strings.Join(m.Features, " "), m.taskText()}, " ")
	// run_api posts every model to /v1/chat/completions. That path is not evidence
	// that an image or video model is a chat LLM, so only text-output models
	// carry the word "chat" from the endpoint.
	endpoint := "messages"
	if m.textOutput() {
		endpoint = "chat completions messages"
	}
	return [6]string{
		lexical(m.ID + " " + vendor),
		lexical(m.displayName()),
		lexical(m.searchSummary()),
		lexical(endpoint),
		lexical(extra),
		lexical(class),
	}
}

func (m LLMModel) pricingBrief() string {
	tiers := pricingTiers(m.Pricing)
	if len(tiers) == 0 {
		return ""
	}
	parts := []string{}
	for _, spec := range []struct{ key, label, mode string }{
		{"prompt", "input", "million"},
		{"completion", "output", "million"},
		{"input_cache_read", "cache read", "million"},
		{"image", "image", "raw"},
		{"request", "per call", "raw"},
		{"duration", "duration", "raw"},
	} {
		amount, ok := formatCatalogAmount(tiers[0][spec.key], spec.mode == "million")
		if !ok {
			continue
		}
		if spec.mode == "million" {
			parts = append(parts, spec.label+" $"+amount+" per 1M tokens")
		} else {
			parts = append(parts, spec.label+" $"+amount)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	brief := strings.Join(parts, ", ")
	if len(tiers) > 1 {
		brief += ". More context tiers are in the catalog."
	}
	return brief
}

func pricingTiers(raw json.RawMessage) []map[string]string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var body any
	if dec.Decode(&body) != nil {
		return nil
	}
	switch v := body.(type) {
	case map[string]any:
		return []map[string]string{priceStrings(v)}
	case []any:
		type tier struct {
			start  *big.Rat
			fields map[string]string
		}
		tiers := []tier{}
		for _, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			tiers = append(tiers, tier{start: ratOf(obj["range_start"]), fields: priceStrings(obj)})
		}
		sort.SliceStable(tiers, func(i, j int) bool {
			if tiers[i].start == nil || tiers[j].start == nil {
				return tiers[i].start != nil
			}
			return tiers[i].start.Cmp(tiers[j].start) < 0
		})
		out := []map[string]string{}
		for _, item := range tiers {
			out = append(out, item.fields)
		}
		return out
	default:
		return nil
	}
}

func priceStrings(obj map[string]any) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"prompt", "completion", "image", "request", "duration", "input_cache_read"} {
		switch v := obj[key].(type) {
		case string:
			out[key] = v
		case json.Number:
			out[key] = v.String()
		}
	}
	return out
}

func ratOf(v any) *big.Rat {
	var text string
	switch n := v.(type) {
	case json.Number:
		text = n.String()
	case string:
		text = n
	default:
		return nil
	}
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(text)); !ok {
		return nil
	}
	return r
}

func formatCatalogAmount(raw string, million bool) (string, bool) {
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(raw)); !ok || r.Sign() <= 0 {
		return "", false
	}
	if million {
		r.Mul(r, big.NewRat(1_000_000, 1))
	}
	text := r.FloatString(12)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "" || text == "0" {
		return "", false
	}
	return text, true
}

func newModelEntry(m LLMModel) modelEntry {
	limitations := []string{"run_api posts this model to /v1/chat/completions. stream true is rejected before the call.", "Catalog pricing is not a price lock."}
	if !m.hasModality("chat") {
		limitations = append(limitations, "The catalog does not list chat as a modality for this model.")
	}
	free := any(nil)
	if m.Free != nil {
		free = *m.Free
	}
	entry := modelEntry{
		model:        m,
		fields:       m.searchFields(),
		summaryText:  m.publicSummary(),
		pricingBrief: m.pricingBrief(),
		limitations:  limitations,
	}
	entry.version = digest([]any{"model", m.ID, m.Name, m.Provider, m.modalityList(), m.Features, m.Sampling, m.ContextLength, m.MaxOutput, string(m.Pricing), free, m.OpenRouterSlug, digest(chatInput.Document)})
	return entry
}

func (e modelEntry) toSummary(reason MatchReason) APISummary {
	m := e.model
	return APISummary{
		Kind:           "model",
		API:            m.ID,
		Title:          short(m.displayName(), 120),
		Provider:       m.Provider,
		Summary:        e.summaryText,
		Method:         "POST",
		Path:           "/v1/chat/completions",
		Categories:     []string{"models", "llm"},
		Capability:     m.primaryCapability(),
		Purpose:        e.summaryText,
		MatchReason:    reason,
		Limitations:    e.limitations,
		RequiredInputs: []string{"messages"},
		ModelID:        m.ID,
		Modalities:     m.modalityList(),
		ContextLength:  m.ContextLength,
		PricingBrief:   e.pricingBrief,
	}
}

func (m LLMModel) primaryCapability() *CapabilityInfo {
	key := ""
	if m.hasModality("chat") {
		key = "chat"
	} else if mods := m.modalityList(); len(mods) > 0 {
		key = mods[0]
	}
	if key == "" {
		return nil
	}
	return &CapabilityInfo{ID: key, Key: key, Name: key}
}

func (e modelEntry) detail() map[string]any {
	m := e.model
	detail := map[string]any{
		"kind": "model", "api": m.ID, "api_id": m.ID, "model_id": m.ID,
		"title": m.displayName(), "provider": m.Provider, "description": e.summaryText,
		"method": "POST", "public_path": "/v1/chat/completions",
		"input_schema": chatInput.Document, "api_version": e.version,
		"categories": []string{"models", "llm"}, "modalities": m.modalityList(),
		"limitations": e.limitations, "required_inputs_brief": []string{"messages"},
	}
	if m.ContextLength > 0 {
		detail["context_length"] = m.ContextLength
	}
	if m.MaxOutput > 0 {
		detail["max_output_length"] = m.MaxOutput
	}
	if len(m.Features) > 0 {
		detail["supported_features"] = append([]string(nil), m.Features...)
	}
	if len(m.Sampling) > 0 {
		detail["supported_sampling_parameters"] = append([]string(nil), m.Sampling...)
	}
	if cap := m.primaryCapability(); cap != nil {
		detail["capability"] = *cap
	}
	if len(m.Pricing) > 0 && string(m.Pricing) != "null" {
		if decoded, err := schema.Decode(m.Pricing); err == nil {
			detail["pricing"] = decoded
		}
		detail["pricing_note"] = pricingNote
	}
	if e.pricingBrief != "" {
		detail["pricing_brief"] = e.pricingBrief
	}
	if m.Free != nil {
		detail["is_free"] = *m.Free
	}
	return detail
}
