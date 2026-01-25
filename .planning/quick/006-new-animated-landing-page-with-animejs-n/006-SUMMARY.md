---
phase: quick
plan: 006
subsystem: documentation
tags: [landing-page, animejs, svg, visualization, hugo]
requires: []
provides: [animated-network-visualization, compact-landing-page]
affects: []
tech-stack:
  added: [animejs@3.2.2]
  patterns: [svg-animation, path-animation]
key-files:
  created:
    - docs-site/static/logos/claude-code.svg
  modified:
    - docs-site/content/en/_index.md
    - docs-site/assets/css/custom.css
decisions: []
metrics:
  duration: 2min
  completed: 2026-01-25
---

# Quick Task 006: Animated Landing Page with AnimeJS Summary

Replaced the verbose 391-line landing page with a compact 277-line version featuring an animated network visualization showing request flow from Claude Code through CC-Relay to multiple providers.

## Changes Made

### New Landing Page (`docs-site/content/en/_index.md`)
- Hero section with title, one-liner subtitle, and two CTA buttons
- SVG-based network diagram showing:
  - Claude Code node (left) with new logo
  - CC-Relay hub (center) with gradient pink/purple circle
  - 5 provider nodes (right) arranged in arc: Anthropic, Z.AI, Ollama, Bedrock, Azure
- 5 animated packets continuously traveling from Claude Code to CC-Relay to random providers
- AnimeJS loaded from CDN for smooth path-following animations
- Compact 4-line terminal quick start
- Compact feature grid showing key stats (6 providers, N keys, 5 strategies, 100% compatible)
- Minimal footer with social links

### CSS Updates (`docs-site/assets/css/custom.css`)
- Added `.network-visualization` container styles
- Added `.network-path` with dashed stroke animation
- Added `.packet` styles for animated request dots
- Added `.features-compact` grid layout
- Added `.terminal-compact` with `.show-immediately` class
- Removed aggressive filter from `.arch-provider-logo` that was breaking Kimi/Moonshot logos
- Added responsive styles for mobile network visualization

### New Asset (`docs-site/static/logos/claude-code.svg`)
- Claude Code logo copied from dashboard-icons

## Technical Implementation

### AnimeJS Animation
- Path-following animation using `getPointAtLength()` for precise movement
- Two-phase animation: main path (Claude Code to CC-Relay), then branch path (CC-Relay to provider)
- Random provider selection on each loop for visual variety
- Staggered packet starts for continuous traffic appearance
- Relay hub pulse animation for emphasis

### SVG Structure
- Quadratic bezier curves for natural-looking paths
- Gradient fills for relay hub and path strokes
- Glow filter for packets and relay hub
- Responsive viewBox for scaling

## Verification

- Hugo builds successfully
- Page size reduced 29% (391 -> 277 lines)
- AnimeJS CDN: https://cdn.jsdelivr.net/npm/animejs@3.2.2/lib/anime.min.js
- Provider logos now render with original colors (filter removed)

## Commits

| Commit | Description |
|--------|-------------|
| e9b6006 | feat(quick-006): add animated network visualization to landing page |

## Deviations from Plan

None - plan executed as written.
