package mcpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

const serverCardPath = "/server-card"

const serverCardMediaType = "application/mcp-server-card+json"

// Documented Server Card schema. The URL is required by the extension schema
// and still returned HTTP 404 from static.modelcontextprotocol.io on 2026-11-26.
const serverCardSchemaURL = "https://static.modelcontextprotocol.io/schemas/v1/server-card.schema.json"

// Published server.json description. It is 98 characters, inside the Server
// Card schema's maxLength of 100. Keep this identical to server.json.
const serverCardDescription = "B2A marketplace: 4,400+ APIs for SEO, social, crypto and leads, plus 168+ AI models. Pay per call."

const serverCardHeaderDescription = "Anonymous catalog discovery and pricing do not authorize charges. Execution uses a Bearer OpenAny API key from secure client settings. This service uses API keys, not OAuth."

type serverCardDocument struct {
	Schema      string             `json:"$schema"`
	Name        string             `json:"name"`
	Title       string             `json:"title"`
	Version     string             `json:"version"`
	Description string             `json:"description"`
	WebsiteURL  string             `json:"websiteUrl"`
	Repository  serverCardRepo     `json:"repository"`
	Remotes     []serverCardRemote `json:"remotes"`
}

type serverCardRepo struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

type serverCardRemote struct {
	Type                      string             `json:"type"`
	URL                       string             `json:"url"`
	Headers                   []serverCardHeader `json:"headers"`
	SupportedProtocolVersions []string           `json:"supportedProtocolVersions"`
}

type serverCardHeader struct {
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	IsRequired  bool                       `json:"isRequired"`
	IsSecret    bool                       `json:"isSecret"`
	Value       string                     `json:"value"`
	Variables   map[string]serverCardInput `json:"variables"`
}

type serverCardInput struct {
	Description string `json:"description"`
	IsRequired  bool   `json:"isRequired"`
	IsSecret    bool   `json:"isSecret"`
}

// openAnyServerCard describes the running https://mcp.openany.si endpoint.
// name is the published registry name because a Server Card name must be
// namespaced. version is the live serverInfo version from the lite server
// (0.1.0), not registry server.json 0.1.5. supportedProtocolVersions matches
// the lite server constructor in lite.go.
var openAnyServerCard = serverCardDocument{
	Schema:      serverCardSchemaURL,
	Name:        "si.openany/openany",
	Title:       "OpenAny",
	Version:     "0.1.0",
	Description: serverCardDescription,
	WebsiteURL:  "https://openany.si",
	Repository: serverCardRepo{
		URL:    "https://github.com/openanysi/openany-mcp",
		Source: "github",
	},
	Remotes: []serverCardRemote{{
		Type: "streamable-http",
		URL:  "https://mcp.openany.si",
		Headers: []serverCardHeader{{
			Name:        "Authorization",
			Description: serverCardHeaderDescription,
			IsRequired:  false,
			IsSecret:    true,
			Value:       "Bearer {OPENANY_API_KEY}",
			Variables: map[string]serverCardInput{
				"OPENANY_API_KEY": {
					Description: "OpenAny API key",
					IsRequired:  false,
					IsSecret:    true,
				},
			},
		}},
		SupportedProtocolVersions: []string{"2026-07-28", "2025-11-25", "2025-06-18", "2025-03-26"},
	}},
}

var (
	serverCardBody []byte
	serverCardETag string
)

func init() {
	var err error
	serverCardBody, err = json.Marshal(openAnyServerCard)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(serverCardBody)
	serverCardETag = `"` + hex.EncodeToString(sum[:]) + `"`
}

func serveServerCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, If-None-Match")
	w.Header().Set("Access-Control-Expose-Headers", "ETag")
	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodGet, http.MethodHead:
	default:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", serverCardMediaType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", serverCardETag)
	if serverCardNoneMatch(r.Header.Get("If-None-Match"), serverCardETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(serverCardBody)))
	if r.Method == http.MethodGet {
		w.Write(serverCardBody)
	}
}

// serverCardNoneMatch applies the weak comparison used by If-None-Match.
func serverCardNoneMatch(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(part)
		if tag == "*" {
			return true
		}
		tag = strings.TrimPrefix(tag, "W/")
		if tag == etag {
			return true
		}
	}
	return false
}
