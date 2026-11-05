package catalog

import (
	"fmt"
	"strings"

	"github.com/openany/openany-mcp/internal/model"
)

type routeKey struct{ provider, method, path string }

func indexRoutes(routes []model.RouteOverride) (map[routeKey]model.Override, error) {
	index := make(map[routeKey]model.Override, len(routes))
	for _, route := range routes {
		if !providerPattern.MatchString(route.ProviderID) || route.Method == "" || !strings.HasPrefix(route.PathTemplate, "/") {
			return nil, fmt.Errorf("invalid route override key")
		}
		key := routeKey{route.ProviderID, route.Method, route.PathTemplate}
		if _, exists := index[key]; exists {
			return nil, fmt.Errorf("duplicate route override")
		}
		index[key] = route.Override
	}
	return index, nil
}

func mergeOverride(base, specific model.Override) model.Override {
	result := base
	if specific.Description != "" {
		result.Description = specific.Description
	}
	if specific.ResponseMode != "" {
		result.ResponseMode = specific.ResponseMode
		result.UnsupportedReason = specific.UnsupportedReason
	} else if specific.UnsupportedReason != "" {
		result.UnsupportedReason = specific.UnsupportedReason
	}
	if len(base.QueryArrays)+len(specific.QueryArrays) > 0 {
		result.QueryArrays = make(map[string]model.ArrayEncoding, len(base.QueryArrays)+len(specific.QueryArrays))
		for name, policy := range base.QueryArrays {
			result.QueryArrays[name] = policy
		}
		for name, policy := range specific.QueryArrays {
			result.QueryArrays[name] = policy
		}
	}
	return result
}
