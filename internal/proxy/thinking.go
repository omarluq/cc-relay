// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"bytes"
	"context"
	"fmt"
	"sort"
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
	var dropMessageIndexes []int

	// Process each message
	messages.ForEach(func(key, msg gjson.Result) bool {
		content, ok := assistantContent(&msg)
		if !ok {
			return true
		}

		// Process content blocks for this assistant message
		var dropMessage bool
		modifiedBody, dropMessage, err = processAssistantContent(
			ctx, modifiedBody, key.Int(), &content, modelName, cache, thinkingCtx,
		)
		if err != nil {
			return false // Stop iteration on error
		}
		if dropMessage {
			dropMessageIndexes = append(dropMessageIndexes, int(key.Int()))
		}

		return true
	})

	if err != nil {
		return body, thinkingCtx, err
	}

	if len(dropMessageIndexes) > 0 {
		modifiedBody, err = dropMessagesByIndex(modifiedBody, dropMessageIndexes)
		if err != nil {
			return body, thinkingCtx, err
		}
	}

	return modifiedBody, thinkingCtx, nil
}

// blockCollector collects and categorizes content blocks during processing.
type blockCollector struct {
	modifiedBlocks []interface{}
	modifiedTypes  []string // Tracks type of each kept block for reordering
}

// thinkingBlockResult holds the outcome of processing a single thinking block.
type thinkingBlockResult struct {
	signature  string
	blockIndex int64
	keep       bool
}

// blockAnalysis holds the results of analyzing content blocks.
type blockAnalysis struct {
	thinkingResults []thinkingBlockResult
	blockTypes      []string
	toolUseIndexes  []int64
	totalBlocks     int
}

// analyzeContentBlocks scans content for thinking/tool_use blocks and their signatures.
// Extracted to reduce cognitive complexity of processAssistantContent.
func analyzeContentBlocks(
	ctx context.Context,
	content *gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
) blockAnalysis {
	var analysis blockAnalysis

	content.ForEach(func(key, block gjson.Result) bool {
		blockType := block.Get("type").String()
		analysis.blockTypes = append(analysis.blockTypes, blockType)
		analysis.totalBlocks++

		switch blockType {
		case blockTypeThinking:
			sig := resolveThinkingSignature(ctx, &block, modelName, cache)
			keep := sig != ""
			if keep {
				thinkingCtx.CurrentSignature = sig
			}
			analysis.thinkingResults = append(analysis.thinkingResults, thinkingBlockResult{
				blockIndex: key.Int(),
				signature:  sig,
				keep:       keep,
			})
		case blockTypeToolUse:
			if block.Get("signature").Exists() {
				analysis.toolUseIndexes = append(analysis.toolUseIndexes, key.Int())
			}
		}
		return true
	})

	return analysis
}

// processAssistantContent processes content blocks in an assistant message.
// Uses surgical in-place edits when possible (only updating signature fields),
// falling back to full content replacement only when blocks need to be dropped
// or reordered. This preserves the original JSON structure byte-for-byte for
// thinking blocks, which the Anthropic API requires.
// countKeptBlocks returns how many blocks will be kept after dropping unsigned thinking.
func (a *blockAnalysis) countKeptBlocks() int {
	kept := a.totalBlocks
	for _, r := range a.thinkingResults {
		if !r.keep {
			kept--
		}
	}
	return kept
}

// rebuildContent rebuilds the content array via the slow path (drops or reordering).
func rebuildContent(
	ctx context.Context,
	body []byte,
	msgIndex int64,
	content *gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
	needsReorder bool,
) ([]byte, error) {
	thinkingCtx.ReorderedBlocks = needsReorder
	collector := &blockCollector{}
	collectBlocks(ctx, content, modelName, cache, thinkingCtx, collector)

	if needsReorder {
		collector.modifiedBlocks = reorderBlocks(collector.modifiedBlocks, collector.modifiedTypes)
	}

	path := fmt.Sprintf("messages.%d.content", msgIndex)
	return sjson.SetBytes(body, path, collector.modifiedBlocks)
}

func processAssistantContent(
	ctx context.Context,
	body []byte,
	msgIndex int64,
	content *gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
) (modifiedBody []byte, dropMessage bool, err error) {
	analysis := analyzeContentBlocks(ctx, content, modelName, cache, thinkingCtx)
	keptCount := analysis.countKeptBlocks()

	if keptCount == 0 {
		thinkingCtx.DroppedBlocks += analysis.totalBlocks - keptCount
		return body, true, nil
	}

	needsDrop := keptCount < analysis.totalBlocks
	needsReorder := !needsDrop && checkReorderNeeded(analysis.blockTypes)

	if !needsDrop && !needsReorder {
		return surgicalUpdate(body, msgIndex, analysis.thinkingResults, analysis.toolUseIndexes)
	}

	newBody, err := rebuildContent(ctx, body, msgIndex, content, modelName, cache, thinkingCtx, needsReorder)
	if err != nil {
		return body, false, fmt.Errorf("failed to set content: %w", err)
	}

	return newBody, false, nil
}

// surgicalUpdate updates only the signature fields in-place without re-serializing blocks.
// This preserves the original JSON structure byte-for-byte, which Anthropic requires.
func surgicalUpdate(
	body []byte,
	msgIndex int64,
	thinkingResults []thinkingBlockResult,
	toolUseIndexes []int64,
) (modifiedBody []byte, dropMessage bool, err error) {
	modifiedBody = body

	// Update thinking block signatures in place
	for _, r := range thinkingResults {
		if !r.keep {
			continue
		}
		path := fmt.Sprintf("messages.%d.content.%d.signature", msgIndex, r.blockIndex)
		modifiedBody, err = sjson.SetBytes(modifiedBody, path, r.signature)
		if err != nil {
			return body, false, fmt.Errorf("failed to update signature: %w", err)
		}
	}

	// Remove signature from tool_use blocks
	for _, idx := range toolUseIndexes {
		path := fmt.Sprintf("messages.%d.content.%d.signature", msgIndex, idx)
		modifiedBody, err = sjson.DeleteBytes(modifiedBody, path)
		if err != nil {
			return body, false, fmt.Errorf("failed to delete tool_use signature: %w", err)
		}
	}

	return modifiedBody, false, nil
}

// resolveThinkingSignature resolves the signature for a thinking block.
// Returns the valid signature, or empty string if block should be dropped.
//
// Priority order:
// 1. Client-provided signature (if valid/prefixed) — most authoritative
// 2. Cached signature — for unsigned blocks from previous responses
// 3. Drop — if no valid signature available.
func resolveThinkingSignature(
	ctx context.Context,
	block *gjson.Result,
	modelName string,
	cache *SignatureCache,
) string {
	thinkingText := block.Get("thinking").String()
	clientSig := block.Get("signature").String()

	// First, try to use the client-provided signature.
	// The client sends back the exact signature we gave them, which is the
	// most authoritative source for this specific thinking block.
	if clientSig != "" {
		// Check if it's a prefixed signature from us
		_, rawSig, ok := ParseSignature(clientSig)
		if ok {
			return rawSig // Strip the prefix, return the original signature
		}
		// Check if it's a valid unprefixed signature
		if IsValidSignature(modelName, clientSig) {
			return clientSig
		}
	}

	// Client didn't provide a valid signature — try cache as fallback.
	// This handles the case where the client sends an unsigned thinking block
	// that we previously signed (cached from the response).
	if cache != nil {
		if cached := cache.Get(ctx, modelName, thinkingText); cached != "" {
			return cached
		}
	}

	// No valid signature available — block should be dropped
	return ""
}

// checkReorderNeeded checks if thinking blocks need to be moved before other blocks.
func checkReorderNeeded(blockTypes []string) bool {
	firstThinking := findFirstIndex(blockTypes, blockTypeThinking)
	firstOther := findFirstNonIndex(blockTypes, blockTypeThinking)
	return firstThinking != -1 && firstOther != -1 && firstOther < firstThinking
}

func assistantContent(msg *gjson.Result) (gjson.Result, bool) {
	if msg.Get("role").String() != "assistant" {
		return gjson.Result{}, false
	}

	content := msg.Get("content")
	if !content.Exists() || !content.IsArray() {
		return gjson.Result{}, false
	}

	return content, true
}

func dropMessagesByIndex(body []byte, indexes []int) ([]byte, error) {
	sort.Sort(sort.Reverse(sort.IntSlice(indexes)))
	modifiedBody := body

	for _, idx := range indexes {
		var err error
		modifiedBody, err = sjson.DeleteBytes(modifiedBody, fmt.Sprintf("messages.%d", idx))
		if err != nil {
			return body, err
		}
	}

	return modifiedBody, nil
}

// collectBlocks iterates content and collects blocks into the collector.
// Used only in the slow path (drops or reordering needed).
func collectBlocks(
	ctx context.Context,
	content *gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
	collector *blockCollector,
) {
	content.ForEach(func(_, block gjson.Result) bool {
		blockType := block.Get("type").String()

		switch blockType {
		case blockTypeThinking:
			processed, keep := processThinkingBlock(ctx, &block, modelName, cache, thinkingCtx)
			if keep {
				collector.modifiedBlocks = append(collector.modifiedBlocks, processed)
				collector.modifiedTypes = append(collector.modifiedTypes, blockTypeThinking)
			}
		case blockTypeToolUse:
			processed := processToolUseBlock(&block, thinkingCtx.CurrentSignature)
			collector.modifiedBlocks = append(collector.modifiedBlocks, processed)
			collector.modifiedTypes = append(collector.modifiedTypes, blockTypeToolUse)
		default:
			collector.modifiedBlocks = append(collector.modifiedBlocks, block.Value())
			collector.modifiedTypes = append(collector.modifiedTypes, blockType)
		}
		return true
	})
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
// Preserves ALL original fields except updating the signature field.
// Used only in the slow path (drops or reordering needed).
func processThinkingBlock(
	ctx context.Context,
	block *gjson.Result,
	modelName string,
	cache *SignatureCache,
	thinkingCtx *ThinkingContext,
) (interface{}, bool) {
	signature := resolveThinkingSignature(ctx, block, modelName, cache)

	// Drop block if no valid signature
	if signature == "" {
		thinkingCtx.DroppedBlocks++
		return nil, false
	}

	// Update current signature for tool_use inheritance
	thinkingCtx.CurrentSignature = signature

	// Preserve entire original block, only updating the signature field.
	// This ensures fields like 'data', 'redacted_thinking', etc. are preserved.
	result := make(map[string]interface{})
	block.ForEach(func(key, value gjson.Result) bool {
		result[key.String()] = value.Value()
		return true
	})
	result["signature"] = signature

	return result, true
}

// processToolUseBlock processes a tool_use block, ensuring no signature field is sent.
func processToolUseBlock(block *gjson.Result, _ string) interface{} {
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
