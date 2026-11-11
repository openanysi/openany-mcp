package catalog

import "sort"

// Overview is computed from the current discovery snapshot.
// It does not read the network or the paid gateway.
func (d *Discovery) Overview() map[string]any {
	byProvider := map[string]int{}
	byCategory := map[string]int{}
	categoryProviders := map[string]map[string]struct{}{}
	for _, name := range d.snapshot.Names {
		if _, ok := d.entries[name]; !ok {
			continue
		}
		provider := d.snapshot.Tools[name].ProviderID
		byProvider[provider]++
		category := ProviderCategory(provider)
		byCategory[category]++
		if categoryProviders[category] == nil {
			categoryProviders[category] = map[string]struct{}{}
		}
		categoryProviders[category][provider] = struct{}{}
	}
	providers := make([]map[string]any, 0, len(byProvider))
	for provider, count := range byProvider {
		providers = append(providers, map[string]any{"provider": provider, "count": count})
	}
	sort.Slice(providers, func(i, j int) bool {
		left, right := providers[i]["count"].(int), providers[j]["count"].(int)
		if left != right {
			return left > right
		}
		return providers[i]["provider"].(string) < providers[j]["provider"].(string)
	})
	categories := make([]map[string]any, 0, len(categoryOrder))
	for _, category := range categoryOrder {
		names := make([]string, 0, len(categoryProviders[category]))
		for provider := range categoryProviders[category] {
			names = append(names, provider)
		}
		sort.Strings(names)
		categories = append(categories, map[string]any{
			"category":  category,
			"count":     byCategory[category],
			"providers": names,
		})
	}
	grouped := map[string][]string{}
	ids := make([]string, 0, len(d.models))
	for id := range d.models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		modality := d.models[id].model.primaryModality()
		grouped[modality] = append(grouped[modality], id)
	}
	modalities := make([]map[string]any, 0, 5)
	for _, modality := range []string{"chat/reasoning", "image", "video", "audio"} {
		modalities = append(modalities, modalityGroup(modality, grouped[modality]))
	}
	if len(grouped["other"]) > 0 {
		modalities = append(modalities, modalityGroup("other", grouped["other"]))
	}
	return map[string]any{
		"api_count":          len(d.entries),
		"model_count":        d.ModelCount,
		"apis_by_provider":   providers,
		"apis_by_category":   categories,
		"models_by_modality": modalities,
		"models_available":   d.ModelsAvailable,
		"models_stale":       d.ModelsStale,
		"example_questions":  d.exampleQuestions(),
		"next_step_hint":     overviewNextStep,
	}
}

func modalityGroup(modality string, ids []string) map[string]any {
	copied := append([]string{}, ids...)
	sort.Strings(copied)
	return map[string]any{"modality": modality, "count": len(copied), "models": copied}
}
