package antigravity

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	js "github.com/santhosh-tekuri/jsonschema/v6"
)

// Official manifest schema fetched 2026-11-17 from
// https://antigravity.google/schemas/v1/plugin.json
// Marketplace checklist: https://antigravity.google/docs/build-plugins/
// MCP server fields: https://antigravity.google/docs/mcp/
const (
	officialLogoSHA256 = "3c38d8ccb36e2eac46c7f024852559c12d35727e6bc116880b626f3f1c333ef4"
	tagline            = "The B2A (business to agent) marketplace"
	endpoint           = "https://mcp.openany.si"
	placeholderHeader  = "Bearer ${OPENANY_API_KEY}"
)

func TestPluginManifestMatchesOfficialSchema(t *testing.T) {
	root := repoRoot(t)
	schemaPath := filepath.Join(root, "internal/antigravity/testdata/plugin.schema.json")
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schemaDoc any
	if err := json.Unmarshal(schemaBytes, &schemaDoc); err != nil {
		t.Fatal(err)
	}
	schemaMap, ok := schemaDoc.(map[string]any)
	if !ok || schemaMap["$id"] != "https://antigravity.google/schemas/v1/plugin.json" {
		t.Fatal("testdata schema is not the official Antigravity plugin manifest schema")
	}
	if !strings.Contains(string(schemaBytes), "(?!") {
		t.Fatal("official schema snapshot lost the logo lookahead pattern")
	}
	// Go's RE2 engine rejects the official logo pattern's lookahead. The file on
	// disk stays verbatim. Validation uses an equivalent relative-path pattern.
	rewriteLookaheadPatterns(schemaDoc)
	compiler := js.NewCompiler()
	compiler.DefaultDraft(js.Draft2020)
	compiler.UseLoader(nil)
	if err := compiler.AddResource("https://antigravity.google/schemas/v1/plugin.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("https://antigravity.google/schemas/v1/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	raw := readRepoFile(t, root, ".antigravity-plugin/plugin.json")
	var manifest any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if err := compiled.Validate(manifest); err != nil {
		t.Fatal(err)
	}

	doc := manifest.(map[string]any)
	if doc["$schema"] != "https://antigravity.google/schemas/v1/plugin.json" {
		t.Fatal("$schema")
	}
	if doc["name"] != "openany" || doc["displayName"] != "OpenAny" {
		t.Fatal("name", doc["name"], doc["displayName"])
	}
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`).MatchString(doc["name"].(string)) {
		t.Fatal("marketplace name must be lowercase kebab-case")
	}
	description := doc["description"].(string)
	if !strings.HasPrefix(description, tagline) {
		t.Fatal("description must start with the tagline")
	}
	if n := len(description); n < 120 || n > 160 {
		t.Fatalf("description length %d, marketplace recommendation is 120-160", n)
	}
	if doc["version"] != "0.1.0" || doc["license"] != "MIT" {
		t.Fatal("version or license")
	}
	if doc["homepage"] != "https://openany.si" || doc["repository"] != "https://github.com/openanysi/openany-mcp" {
		t.Fatal("homepage or repository")
	}
	if doc["logo"] != "assets/logo.png" {
		t.Fatal("logo path")
	}
	author := doc["author"].(map[string]any)
	if author["email"] != "team@openany.si" || author["url"] != "https://openany.si" {
		t.Fatal("author contact")
	}
	name := author["name"].(string)
	for _, part := range []string{"Pistis Wang", "OpenAny", "Alephant AI LLC"} {
		if !strings.Contains(name, part) {
			t.Fatal("author name missing", part)
		}
	}
	prompts := doc["suggestedPrompts"].([]any)
	if len(prompts) == 0 || len(prompts) > 3 {
		t.Fatal("suggestedPrompts", len(prompts))
	}
	keywords := doc["keywords"].([]any)
	joined := strings.Join(stringList(keywords), " ")
	for _, word := range []string{"social", "seo", "search", "crypto", "company", "leads", "finance", "legal", "gpt", "claude", "gemini", "deepseek", "qwen", "midjourney", "veo", "kling"} {
		if !strings.Contains(joined, word) {
			t.Fatal("keyword missing", word)
		}
	}
	assertSquareLogo(t, filepath.Join(root, "assets/logo.png"))
	if strings.Contains(string(raw), "/mcp\"") || strings.Contains(description, "/mcp") {
		t.Fatal("manifest must not point at the /mcp catalog")
	}
}

func TestMCPConfigMatchesDocumentedRemoteServer(t *testing.T) {
	root := repoRoot(t)
	raw := readRepoFile(t, root, "mcp_config.json")
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc) != 1 {
		t.Fatal("mcp_config root keys", len(doc))
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok || len(servers) != 1 {
		t.Fatal("mcpServers")
	}
	server, ok := servers["openany"].(map[string]any)
	if !ok {
		t.Fatal("openany server")
	}
	allowed := map[string]bool{
		"serverUrl": true, "headers": true, "variables": true,
	}
	for key := range server {
		if !allowed[key] {
			t.Fatal("unexpected server field", key)
		}
	}
	if server["serverUrl"] != endpoint {
		t.Fatal("serverUrl", server["serverUrl"])
	}
	headers := server["headers"].(map[string]any)
	if len(headers) != 1 || headers["Authorization"] != placeholderHeader {
		t.Fatal("headers", headers)
	}
	variables := server["variables"].([]any)
	if len(variables) != 1 {
		t.Fatal("variables")
	}
	variable := variables[0].(map[string]any)
	for key := range variable {
		switch key {
		case "name", "title", "description", "link":
		default:
			t.Fatal("unexpected variable field", key)
		}
	}
	if variable["name"] != "OPENANY_API_KEY" || variable["link"] != "https://openany.si/dashboard?page=api-keys" {
		t.Fatal("variable", variable["name"], variable["link"])
	}
	blob := string(raw)
	if strings.Contains(blob, endpoint+"/mcp") || strings.Contains(blob, "oauth") || strings.Contains(blob, "\"command\"") {
		t.Fatal("mcp_config must stay a keyless-url remote server with a placeholder header")
	}
	rejectSecrets(t, blob)
}

func TestPluginReadmeDocumentsInstallAndAuth(t *testing.T) {
	root := repoRoot(t)
	readme := string(readRepoFile(t, root, ".antigravity-plugin/README.md"))
	for _, phrase := range []string{
		tagline,
		endpoint,
		"agy plugin install .",
		"agy plugin validate .",
		placeholderHeader,
		"OPENANY_API_KEY",
		"https://openany.si/dashboard?page=api-keys",
		"https://openany.si/docs/",
		"team@openany.si",
		"Pistis Wang",
		"Alephant AI LLC",
		"pay per call",
		"no monthly fee",
		"https://mcp.openany.si/mcp",
		"disabled until",
	} {
		if !strings.Contains(readme, phrase) {
			t.Fatal("readme missing", phrase)
		}
	}
	if strings.Contains(readme, "agy plugin validate --strict .") && strings.Contains(readme, "passes") {
		t.Fatal("readme must not claim strict validation passed")
	}
	rejectSecrets(t, readme)
}

func rewriteLookaheadPatterns(v any) {
	switch node := v.(type) {
	case map[string]any:
		if pattern, ok := node["pattern"].(string); ok && strings.Contains(pattern, "(?!") {
			node["pattern"] = `^[^/:].+\.(png|svg|jpg|jpeg|webp|PNG|SVG|JPG|JPEG|WEBP)$`
		}
		for _, child := range node {
			rewriteLookaheadPatterns(child)
		}
	case []any:
		for _, child := range node {
			rewriteLookaheadPatterns(child)
		}
	}
}

func assertSquareLogo(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != officialLogoSHA256 {
		t.Fatal("logo is not the official 1200x1200 OpenAny asset")
	}
	if len(data) < 24 || !bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatal("logo is not a PNG")
	}
	if !bytes.Equal(data[12:16], []byte("IHDR")) {
		t.Fatal("PNG IHDR")
	}
	width := binary.BigEndian.Uint32(data[16:20])
	height := binary.BigEndian.Uint32(data[20:24])
	if width != height || width < 128 {
		t.Fatalf("logo size %dx%d", width, height)
	}
}

func rejectSecrets(t *testing.T, text string) {
	t.Helper()
	for _, pattern := range []string{"sk-", "sk_live", "sk_test", "AKIA", "BEGIN PRIVATE"} {
		if strings.Contains(text, pattern) {
			t.Fatal("secret-like material", pattern)
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "Bearer ") && !strings.Contains(line, placeholderHeader) && !strings.Contains(line, "Bearer <key>") && !strings.Contains(line, "Bearer token") {
			t.Fatal("authorization line is not the placeholder", line)
		}
	}
}

func stringList(values []any) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = value.(string)
	}
	return out
}

func readRepoFile(t *testing.T, root, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
