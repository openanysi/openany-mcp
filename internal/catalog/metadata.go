package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openany/openany-mcp/internal/schema"
)

type CapabilityInfo struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}
type PublicSample struct {
	Name          string         `json:"name"`
	Request       map[string]any `json:"request"`
	HTTPStatus    int            `json:"http_status"`
	ResponseShape map[string]any `json:"response_shape"`
}
type APIMetadata struct {
	Revision                int               `json:"revision"`
	Categories              []string          `json:"categories"`
	Platform                string            `json:"data_platform"`
	Product                 string            `json:"data_product"`
	Capability              CapabilityInfo    `json:"capability"`
	NameTranslations        map[string]string `json:"name_translations"`
	DescriptionTranslations map[string]string `json:"description_translations"`
	CapabilityTranslations  map[string]string `json:"capability_translations"`
	Samples                 []PublicSample    `json:"samples"`
	Available               *bool             `json:"available"`
}
type MetadataLoader func(context.Context) (map[string]APIMetadata, error)

// This projection deliberately excludes upstream credentials, mappings, fixed bodies and costs.
const MetadataQuery = `SELECT a.id::text,jsonb_build_object(
 'revision',a.revision,'categories',a.categories,'data_platform',COALESCE(a.data_platform,''),'data_product',COALESCE(a.data_product,''),
 'name_translations',a.name_translations,'description_translations',a.description_translations,
 'capability',jsonb_build_object('id',cap.id::text,'key',cap.key,'name',cap.name,'description',cap.description),
 'capability_translations',cap.name_translations,
 'available',COALESCE(k.status::text='active' AND (a.capability_id IS NULL OR cap.status='active'),false),
 'samples',COALESCE((SELECT jsonb_agg(jsonb_build_object('name',v->'name','request',v->'request','http_status',v->'http_status',
 'response_shape',jsonb_build_object('type',jsonb_typeof(v->'response_body'),
 'fields',CASE WHEN jsonb_typeof(v->'response_body')='object' THEN (SELECT jsonb_agg(left(key,80)) FROM (SELECT key FROM jsonb_object_keys(v->'response_body') AS key ORDER BY key LIMIT 12) f) ELSE NULL END,
 'items',CASE WHEN jsonb_typeof(v->'response_body')='array' THEN jsonb_array_length(v->'response_body') ELSE NULL END)))
 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(c.samples)='array' THEN c.samples ELSE '[]'::jsonb END) WITH ORDINALITY AS sample(v,n)
 WHERE n<=3 AND octet_length((v->'request')::text)<=16384),'[]'::jsonb))
 FROM public.agent_apis a
 LEFT JOIN public.agent_api_request_configs c ON c.api_id=a.id
 LEFT JOIN public.agent_api_capabilities cap ON cap.id=a.capability_id
 LEFT JOIN public.key_market_upstream_keys k ON k.id=a.upstream_key_id
 WHERE a.status='active'`

func PostgresMetadataLoader(pool *pgxpool.Pool, timeout time.Duration) MetadataLoader {
	return func(ctx context.Context) (map[string]APIMetadata, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		rows, err := pool.Query(ctx, MetadataQuery)
		if err != nil {
			return nil, fmt.Errorf("public metadata read failed")
		}
		defer rows.Close()
		out := map[string]APIMetadata{}
		for rows.Next() {
			var id string
			var raw []byte
			if rows.Scan(&id, &raw) != nil {
				return nil, fmt.Errorf("public metadata decode failed")
			}
			// Preserve exact numeric values in request samples.
			v, err := schema.Decode(raw)
			if err != nil {
				return nil, fmt.Errorf("public metadata decode failed")
			}
			data, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("public metadata decode failed")
			}
			sampleData := data["samples"]
			delete(data, "samples")
			safe, _ := json.Marshal(data)
			var m APIMetadata
			if json.Unmarshal(safe, &m) != nil {
				continue
			}
			if samples, ok := sampleData.([]any); ok {
				for _, value := range samples {
					obj, ok := value.(map[string]any)
					if !ok {
						continue
					}
					request, _ := obj["request"].(map[string]any)
					name, _ := obj["name"].(string)
					status := 0
					if n, ok := obj["http_status"].(json.Number); ok {
						a, _ := n.Int64()
						status = int(a)
					}
					shape, _ := obj["response_shape"].(map[string]any)
					m.Samples = append(m.Samples, PublicSample{name, request, status, shape})
				}
			}
			out[id] = m
		}
		if rows.Err() != nil {
			return nil, fmt.Errorf("public metadata read failed")
		}
		return out, nil
	}
}

var credentialText = regexp.MustCompile(`(?i)(bearer\s|api[_ -]?key|authorization\s*:|密钥|supplier[._ ]?cost)`)

func publicText(s string, max int) string {
	lines := []string{}
	for _, line := range strings.Split(s, "\n") {
		if !credentialText.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return short(strings.Join(lines, " "), max)
}
func localized(values map[string]string) string {
	for _, key := range []string{"zh-CN", "zh", "zh-Hans", "zh_CN", "en", "en-US"} {
		if s := publicText(values[key], 180); s != "" {
			return s
		}
	}
	return ""
}
func publicCategories(m APIMetadata) []string {
	out := append([]string{}, m.Categories...)
	p := strings.ToLower(strings.TrimSpace(m.Platform))
	if (p == "chatgpt" || p == "gemini") && len(out) < 16 {
		found := false
		for _, c := range out {
			found = found || c == "ai-generation"
		}
		if !found {
			out = append(out, "ai-generation")
		}
	}
	if len(out) > 16 {
		out = out[:16]
	}
	return out
}
