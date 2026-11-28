package mcpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/openany/openany-mcp/internal/account"
)

func TestExposureServerCardPreservesKeywordsAndBenefits(t *testing.T) {
	raw, err := os.ReadFile("../../server.json")
	if err != nil {
		t.Fatal(err)
	}
	var registry struct{ Description string }
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	if registry.Description != serverCardDescription {
		t.Fatal("registry and base Server Card descriptions differ")
	}
	for _, enabled := range []bool{false, true} {
		name := "base"
		if enabled {
			name = "benefits"
		}
		t.Run(name, func(t *testing.T) {
			h := newServerCardHandler(t, Options{LiteEnabled: true, BenefitsEnabled: enabled})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://localhost/server-card", nil))
			var card serverCardDocument
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &card) != nil {
				t.Fatal("server card unavailable", w.Code, w.Body.String())
			}
			if len(card.Description) > 100 || strings.Contains(card.Description, "4,400") || strings.Contains(card.Description, "168+") || strings.Contains(card.Description, "$0.10") {
				t.Fatal("unbounded or fixed promise in description", card.Description)
			}
			for _, keyword := range []string{"SEO", "social", "crypto", "lead", "AI models"} {
				if !strings.Contains(card.Description, keyword) {
					t.Fatal("main exposure keyword lost", keyword)
				}
			}
			if enabled {
				if card.Description != serverCardBenefits || !strings.Contains(card.Description, "Trial and referral") || w.Header().Get("ETag") != serverCardBenefitsETag || serverCardBenefitsETag == serverCardETag {
					t.Fatal("benefits description/cache separation lost")
				}
			} else if card.Description != registry.Description || w.Header().Get("ETag") != serverCardETag {
				t.Fatal("base card no longer matches registry")
			}
		})
	}
}

func TestExposureDefaultFiveToolsKeepRoutingCopy(t *testing.T) {
	client, err := account.NewClient("http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	h := New(mergedGuideManager(t), nil, Options{LiteEnabled: true, AccountClient: client})
	for _, path := range []string{"/", "/mcp-lite"} {
		t.Run(path, func(t *testing.T) {
			status, reply, _ := onboardingRPC(t, h, path, "tools/list", map[string]any{}, nil)
			if status != http.StatusOK || reply["error"] != nil {
				t.Fatal("tools/list failed", status, reply)
			}
			tools := reply["result"].(map[string]any)["tools"].([]any)
			if len(tools) != 5 {
				t.Fatal("default five-tool contract lost", len(tools))
			}
			for _, item := range tools {
				tool := item.(map[string]any)
				description := tool["description"].(string)
				if strings.Contains(description, "4,400") || strings.Contains(description, "168+") {
					t.Fatal("tool description uses a fixed catalog count", tool["name"])
				}
				if tool["name"] == "search_apis" {
					for _, keyword := range []string{"openany_overview", "DataForSEO", "Xiaohongshu", "小红书", "Douyin", "抖音", "Weibo", "微博", "Hunter", "CoinGecko", "get_api", "run_api"} {
						if !strings.Contains(description, keyword) {
							t.Fatal("routing keyword or continuation lost", keyword)
						}
					}
				}
			}
		})
	}
}
