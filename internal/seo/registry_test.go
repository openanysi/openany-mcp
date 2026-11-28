package seo

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRegistryDescriptionIsTheApprovedSentence(t *testing.T) {
	if len(RegistryDescription) != 94 {
		t.Fatalf("description length %d", len(RegistryDescription))
	}
	raw, err := os.ReadFile("../../seo/server.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Name, Title, Description, Version string
		Remotes                           []struct {
			URL string `json:"url"`
		} `json:"remotes"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		t.Fatal("seo/server.json is not the candidate descriptor")
	}
	if doc.Name != RegistryName || doc.Title != RegistryTitle || doc.Description != RegistryDescription || doc.Version != ServerVersion {
		t.Fatal(doc)
	}
	if len(doc.Remotes) != 1 || doc.Remotes[0].URL != Endpoint {
		t.Fatal(doc.Remotes)
	}
	for _, banned := range []string{"Ahrefs", "Semrush", "backlinks", "Domain Rating", "Serper", "Tavily", "search_apis", "run_api"} {
		if strings.Contains(doc.Description, banned) {
			t.Fatal("registry description claims or names", banned)
		}
	}
}

func TestCopyRoutesToolsWithoutCatalogTools(t *testing.T) {
	for _, text := range []string{Instructions, KeywordResearchDescription, KeywordIdeasDescription, RankedKeywordsDescription, SERPOverviewDescription} {
		if !strings.Contains(text, "Use when") || !strings.Contains(text, "Do NOT") {
			t.Fatal("tool copy lost Use when / Do NOT")
		}
		for _, banned := range []string{"search_apis", "get_api", "run_api", "openany_overview"} {
			if strings.Contains(text, banned) {
				t.Fatal("vertical copy names a root catalog tool", banned)
			}
		}
	}
	if !strings.Contains(Guide, "这个词该不该做") || strings.Contains(RegistryDescription, "这个词该不该做") {
		t.Fatal("Chinese hook moved into the registry description or left the human guide")
	}
	if !strings.Contains(KeywordResearchDescription, "DataForSEO") || !strings.Contains(RankedKeywordsDescription, "Google-only") {
		t.Fatal("source or engine boundary missing")
	}
}
