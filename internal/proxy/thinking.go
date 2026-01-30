// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Block type constants.
const (
	blockTypeThinking = "thinking"
	blockTypeToolUse  = "tool_use"
)

// Detection markers for thinking blocks (used for fast path detection).
// We check for both compact and spaced JSON formats.
var (
	thinkingTypeMarker       = []byte(`"type":"thinking"`)
	thinkingTypeMarkerSpaced = []byte(`"type": "thinking"`)
	thinkingFieldMarker      = []byte(`"thinking"`)
	signatureMarker          = []byte(`"signature"`)
)

// ThinkingContext holds state for processing thinking blocks in a message.
type ThinkingContext struct {
	CurrentSignature        string
	AccumulatedThinkingText strings.Builder
	DroppedBlocks           int
	ReorderedBlocks         bool
}

// HasThinkingBlocks performs fast detection without JSON parsing.
// Returns true if the body likely contains thinking blocks with signatures.
// Uses bytes.Contains which is 10-100x faster than JSON parsing.
func HasThinkingBlocks(body []byte) bool {
	hasThinkingType := bytes.Contains(body, thinkingTypeMarker) ||
		bytes.Contains(body, thinkingTypeMarkerSpaced)
	return hasThinkingType &&
		bytes.Contains(body, thinkingFieldMarker) &&
		bytes.Contains(body, signatureMarker)
}

// ProcessRequestThinking processes thinking blocks in the request body:
// 1. Looks up cached signatures for thinking blocks
// 2. Falls back to client-provided signature (validating format)
// 3. Drops unsigned thinking blocks
// 4. Tracks signature for tool_use inheritance
// 5. Reorders blocks so thinking comes first
//
// Returns modified body and context, or error.
func ProcessRequestThinking(
	ctx context.Context,
	body []byte,
	modelName string,
	cache *SignatureCache,
) ([]byte, *ThinkingContext, error) {
	thinkingCtx := &ThinkingContext{}

	// Parse messages array
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body, thinkingCtx, nil
	}

	modifiedBody := body
	var err error

	// Process each message
	messages.ForEach(func(key, msg gjson.Result) bool {
		role := msg.Get("role").String()
		if role != "assistant" {
			return true // Continue to next message
		}

		content := msg.Get("content")
		if !content.Exists() || !content.IsArray() {
			return true
		}

		// Process content blocks for this assistant message
		modifiedBody, err = processAssistantContent(
			ctx, modifiedBody, key.Int(), content, modelName, cache, thinkingCtx,
		)
		if err != nil {
			return false // Stop iteration on error
		}

		return true
	})

	if err != nil {
		return body, thinkingCtx, err
	}

	return modifiedBody, thinkingCtx, nil
}

// blockCollector collects and categorizes content blocks during processing.
type blockCollector struct {
	modifiedBlocks []interface{}
	modifiedTypes  []string // Tracks type of each kept block for reordering
}

// processAssistantContent processes content blocks in an assistant message.
//
//nolint:gocritic // hugeParam: gjson.Result is passed by value by design
func processAssistantContent(
	ctx context.Context,
	body []byte,
	msgIndex int64,
	content gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
) ([]byte, error) {
	collector := &blockCollector{}
	collectBlocks(ctx, content, modelName, cache, thinkingCtx, collector)

	// Check if reordering is needed (uses tracked types, not original content)
	if needsReordering(collector) {
		thinkingCtx.ReorderedBlocks = true
		collector.modifiedBlocks = reorderBlocks(collector.modifiedBlocks, collector.modifiedTypes)
	}

	// Update the content array in the body
	path := fmt.Sprintf("messages.%d.content", msgIndex)
	newBody, err := sjson.SetBytes(body, path, collector.modifiedBlocks)
	if err != nil {
		return body, fmt.Errorf("failed to set content: %w", err)
	}

	return newBody, nil
}

// collectBlocks iterates content and collects blocks into the collector.
//
//nolint:gocritic // hugeParam: gjson.Result is passed by value by design
func collectBlocks(
	ctx context.Context,
	content gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
	collector *blockCollector,
) {
	content.ForEach(func(_, block gjson.Result) bool {
		blockType := block.Get("type").String()

		switch blockType {
		case blockTypeThinking:
			processed, keep := processThinkingBlock(ctx, block, modelName, cache, thinkingCtx)
			if keep {
				collector.modifiedBlocks = append(collector.modifiedBlocks, processed)
				collector.modifiedTypes = append(collector.modifiedTypes, blockTypeThinking)
			}
		case blockTypeToolUse:
			processed := processToolUseBlock(block, thinkingCtx.CurrentSignature)
			collector.modifiedBlocks = append(collector.modifiedBlocks, processed)
			collector.modifiedTypes = append(collector.modifiedTypes, blockTypeToolUse)
		default:
			collector.modifiedBlocks = append(collector.modifiedBlocks, block.Value())
			collector.modifiedTypes = append(collector.modifiedTypes, blockType)
		}
		return true
	})
}

// needsReordering checks if thinking blocks need to be moved before other blocks.
// Uses tracked types from collector instead of re-parsing original content.
func needsReordering(collector *blockCollector) bool {
	firstThinkingIdx := findFirstIndex(collector.modifiedTypes, blockTypeThinking)
	firstOtherIdx := findFirstNonIndex(collector.modifiedTypes, blockTypeThinking)

	// Only reorder if we have both types and other comes before thinking
	return firstThinkingIdx != -1 && firstOtherIdx != -1 && firstOtherIdx < firstThinkingIdx
}

// findFirstIndex returns the index of the first occurrence of target, or -1.
func findFirstIndex(types []string, target string) int {
	for i, t := range types {
		if t == target {
			return i
		}
	}
	return -1
}

// findFirstNonIndex returns the index of the first element not matching target, or -1.
func findFirstNonIndex(types []string, target string) int {
	for i, t := range types {
		if t != target {
			return i
		}
	}
	return -1
}

// reorderBlocks moves thinking blocks before other blocks.
// Preserves relative order within each group and runs in O(n).
func reorderBlocks(blocks []interface{}, types []string) []interface{} {
	thinking := make([]interface{}, 0, len(blocks))
	other := make([]interface{}, 0, len(blocks))

	for i, t := range types {
		if t == blockTypeThinking {
			thinking = append(thinking, blocks[i])
		} else {
			other = append(other, blocks[i])
		}
	}

	return append(thinking, other...)
}

// processThinkingBlock processes a single thinking block.
// Returns the processed block value and whether to keep it.
//
//nolint:gocritic // hugeParam: gjson.Result is passed by value by design
func processThinkingBlock(
	ctx context.Context,
	block gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
) (interface{}, bool) {
	thinkingText := block.Get("thinking").String()
	clientSig := block.Get("signature").String()

	// Try to get cached signature first
	var signature string
	if cache != nil {
		signature = cache.Get(ctx, modelName, thinkingText)
	}

	// Fall back to client signature if cache miss
	if signature == "" {
		// Parse client signature (may have model group prefix)
		_, rawSig, ok := ParseSignature(clientSig)
		if ok {
			signature = rawSig
		} else if IsValidSignature(modelName, clientSig) {
			signature = clientSig
		}
	}

	// Drop block if no valid signature
	if signature == "" {
		thinkingCtx.DroppedBlocks++
		return nil, false
	}

	// Update current signature for tool_use inheritance
	thinkingCtx.CurrentSignature = signature

	// Create modified block with signature
	result := map[string]interface{}{
		"type":      blockTypeThinking,
		"thinking":  thinkingText,
		"signature": signature,
	}

	return result, true
}

// processToolUseBlock processes a tool_use block, ensuring no signature field is sent.
//
//nolint:gocritic // hugeParam: gjson.Result is passed by value by design
func processToolUseBlock(block gjson.Result, _ string) interface{} {
	result := make(map[string]interface{})

	// Copy all fields from original block
	block.ForEach(func(key, value gjson.Result) bool {
		result[key.String()] = value.Value()
		return true
	})

	delete(result, "signature")

	return result
}

// ProcessResponseSignature handles signature_delta events from upstream.
// Extracts signature, caches it, and transforms to include modelGroup prefix.
// Returns the modified event data with prefixed signature.
func ProcessResponseSignature(
	ctx context.Context,
	eventData []byte,
	thinkingText string,
	modelName string,
	cache *SignatureCache,
) []byte {
	// Extract signature from event data
	signature := gjson.GetBytes(eventData, "delta.signature").String()
	if signature == "" {
		signature = gjson.GetBytes(eventData, "signature").String()
	}

	if signature == "" {
		return eventData
	}

	// Cache the signature
	if cache != nil && thinkingText != "" {
		cache.Set(ctx, modelName, thinkingText, signature)
	}

	// Add model group prefix to signature
	prefixedSig := FormatSignature(modelName, signature)

	// Update signature in event data
	var modifiedData []byte
	var err error

	if gjson.GetBytes(eventData, "delta.signature").Exists() {
		modifiedData, err = sjson.SetBytes(eventData, "delta.signature", prefixedSig)
	} else {
		modifiedData, err = sjson.SetBytes(eventData, "signature", prefixedSig)
	}

	if err != nil {
		return eventData
	}

	return modifiedData
}

// FormatSignature adds modelGroup prefix: "claude#abc123...".
func FormatSignature(modelName, signature string) string {
	return fmt.Sprintf("%s#%s", GetModelGroup(modelName), signature)
}

// ParseSignature extracts modelGroup and raw signature from prefixed format.
// Returns modelGroup, signature, ok.
func ParseSignature(prefixed string) (modelGroup, signature string, ok bool) {
	idx := strings.Index(prefixed, "#")
	if idx == -1 {
		return "", "", false
	}
	return prefixed[:idx], prefixed[idx+1:], true
}

// ProcessNonStreamingResponse processes thinking blocks in a non-streaming response.
// Extracts and caches signatures, adds modelGroup prefix to signatures.
func ProcessNonStreamingResponse(
	ctx context.Context,
	body []byte,
	modelName string,
	cache *SignatureCache,
) []byte {
	content := gjson.GetBytes(body, "content")
	if !content.Exists() || !content.IsArray() {
		return body
	}

	modifiedBody := body

	content.ForEach(func(key, block gjson.Result) bool {
		blockType := block.Get("type").String()
		if blockType != blockTypeThinking {
			return true
		}

		thinkingText := block.Get(blockTypeThinking).String()
		signature := block.Get("signature").String()

		if signature == "" {
			return true
		}

		// Cache the signature
		if cache != nil && thinkingText != "" {
			cache.Set(ctx, modelName, thinkingText, signature)
		}

		// Add model group prefix
		prefixedSig := FormatSignature(modelName, signature)
		path := fmt.Sprintf("content.%d.signature", key.Int())

		var err error
		modifiedBody, err = sjson.SetBytes(modifiedBody, path, prefixedSig)
		if err != nil {
			return true // Continue on error
		}

		return true
	})

	return modifiedBody
}
