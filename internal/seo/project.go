package seo

// Project pulls the fields this vertical promises out of a DataForSEO payload.
// When the shape is not recognized, ok is false and the caller keeps the payload.
func Project(api string, data any) (any, bool) {
	rows := resultRows(data)
	if rows == nil {
		return nil, false
	}
	switch api {
	case GoogleHistorical:
		return map[string]any{"keywords": projectHistory(rows), "count": len(rows), "note": "Volume history for the keywords submitted. This is not a list of newly discovered keywords."}, true
	case GoogleOverview, GoogleIdeas, GoogleRelated, GoogleForSite:
		return map[string]any{"keywords": projectKeywords(rows), "count": len(rows)}, true
	case BingVolume:
		return map[string]any{"keywords": projectBingVolume(rows), "keyword_difficulty": nil, "count": len(rows), "note": "Bing v1 returns search volume only. Keyword difficulty is not part of this response."}, true
	case GoogleRanked:
		return map[string]any{"ranked_keywords": projectRanked(rows), "count": len(rows)}, true
	case GoogleSERP, BingSERP:
		organic, other := projectSERP(rows)
		return map[string]any{"organic": organic, "other_results": other, "organic_count": len(organic)}, true
	default:
		return nil, false
	}
}

func resultRows(data any) []any {
	root, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	tasks, _ := root["tasks"].([]any)
	if len(tasks) == 0 {
		return nil
	}
	task, _ := tasks[0].(map[string]any)
	results, _ := task["result"].([]any)
	if len(results) == 0 {
		return nil
	}
	if first, ok := results[0].(map[string]any); ok {
		if items, ok := first["items"].([]any); ok {
			return items
		}
	}
	return results
}

func projectKeywords(rows []any) []any {
	out := make([]any, 0, len(rows))
	for _, raw := range rows {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		info, _ := item["keyword_info"].(map[string]any)
		props, _ := item["keyword_properties"].(map[string]any)
		row := map[string]any{"keyword": firstString(item["keyword"], info["keyword"])}
		copyIf(row, "search_volume", info["search_volume"], item["search_volume"])
		copyIf(row, "competition", info["competition"], item["competition"])
		copyIf(row, "cpc", info["cpc"], item["cpc"])
		if props["keyword_difficulty"] != nil {
			row["keyword_difficulty"] = props["keyword_difficulty"]
		} else if item["keyword_difficulty"] != nil {
			row["keyword_difficulty"] = item["keyword_difficulty"]
		}
		if info["monthly_searches"] != nil {
			row["monthly_searches"] = info["monthly_searches"]
		}
		out = append(out, row)
	}
	return out
}

func projectHistory(rows []any) []any {
	out := make([]any, 0, len(rows))
	for _, raw := range rows {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		row := map[string]any{"keyword": item["keyword"]}
		copyIf(row, "history", item["history"])
		copyIf(row, "keyword_info", item["keyword_info"])
		out = append(out, row)
	}
	return out
}

func projectBingVolume(rows []any) []any {
	out := make([]any, 0, len(rows))
	for _, raw := range rows {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		row := map[string]any{"keyword": item["keyword"], "keyword_difficulty": nil}
		copyIf(row, "search_volume", item["search_volume"])
		copyIf(row, "competition", item["competition"])
		out = append(out, row)
	}
	return out
}

func projectRanked(rows []any) []any {
	out := make([]any, 0, len(rows))
	for _, raw := range rows {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		keywordData, _ := item["keyword_data"].(map[string]any)
		info, _ := keywordData["keyword_info"].(map[string]any)
		element, _ := item["ranked_serp_element"].(map[string]any)
		serp, _ := element["serp_item"].(map[string]any)
		row := map[string]any{"keyword": firstString(keywordData["keyword"], item["keyword"])}
		copyIf(row, "search_volume", info["search_volume"])
		copyIf(row, "rank_group", serp["rank_group"])
		copyIf(row, "rank_absolute", serp["rank_absolute"])
		copyIf(row, "url", serp["url"])
		copyIf(row, "title", serp["title"])
		out = append(out, row)
	}
	return out
}

func projectSERP(rows []any) (organic, other []any) {
	organic, other = []any{}, []any{}
	for _, raw := range rows {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		row := map[string]any{}
		copyIf(row, "type", item["type"])
		copyIf(row, "rank_group", item["rank_group"])
		copyIf(row, "title", item["title"])
		copyIf(row, "url", item["url"])
		copyIf(row, "description", item["description"])
		if item["type"] == "organic" {
			organic = append(organic, row)
			continue
		}
		if item["title"] != nil || item["url"] != nil {
			other = append(other, row)
		}
	}
	return organic, other
}

func firstString(values ...any) any {
	for _, value := range values {
		if s, ok := value.(string); ok && s != "" {
			return s
		}
	}
	return nil
}

func copyIf(dst map[string]any, key string, values ...any) {
	for _, value := range values {
		if value != nil {
			dst[key] = value
			return
		}
	}
}
