// Package proxy implements the HTTP reverse proxy for Claude Code.
package proxy

import (
	"strings"

	"github.com/omarluq/cc-relay/internal/router"
	"github.com/samber/lo"
	"github.com/samber/mo"
)

// FilterProvidersByModel returns providers that can serve the given model.
// Uses prefix matching on model names (e.g., "claude-opus" matches "claude-opus-4").
//
// Parameters:
//   - model: The model name from the request (e.g., "claude-opus-4")
//   - providers: All available providers
//   - modelMapping: Map of model prefix to provider name (e.g., "claude-opus" -> "anthropic")
//   - defaultProviderName: Fallback provider if no mapping matches
//
// Returns:
//   - Filtered providers that match the model, or all providers if no filtering applies
//
// Behavior:
//   - If model is empty or modelMapping is empty, returns all providers
//   - Uses longest prefix match for specificity
//   - Falls back to defaultProviderName if no prefix matches
//   - Returns all providers if neither match nor default found
func FilterProvidersByModel(
	model string,
	providers []router.ProviderInfo,
	modelMapping map[string]string,
	defaultProviderName string,
) []router.ProviderInfo {
	// No filtering if model is empty or no mapping configured
	if model == "" || len(modelMapping) == 0 {
		return providers
	}

	// Find target provider name using longest prefix match
	targetProviderName := findProviderForModel(model, modelMapping).
		OrElse(defaultProviderName)

	// No filtering possible if no target provider
	if targetProviderName == "" {
		return providers
	}

	// Filter providers by name
	filtered := lo.Filter(providers, func(p router.ProviderInfo, _ int) bool {
		return p.Provider.Name() == targetProviderName
	})

	// If filtering results in empty list, return all providers (graceful degradation)
	if len(filtered) == 0 {
		return providers
	}

	return filtered
}

// findProviderForModel finds the provider name for a model using prefix matching.
// Uses longest prefix match for specificity (e.g., "claude-opus" beats "claude").
// Returns mo.None if no match found, mo.Some with provider name otherwise.
func findProviderForModel(model string, modelMapping map[string]string) mo.Option[string] {
	// Convert map to entries and filter to matching prefixes
	entries := lo.Entries(modelMapping)

	matches := lo.Filter(entries, func(e lo.Entry[string, string], _ int) bool {
		return strings.HasPrefix(model, e.Key)
	})

	// No matches found
	if len(matches) == 0 {
		return mo.None[string]()
	}

	// Find the longest prefix match using lo.MaxBy
	longest := lo.MaxBy(matches, func(a, b lo.Entry[string, string]) bool {
		return len(a.Key) > len(b.Key)
	})

	return mo.Some(longest.Value)
}
