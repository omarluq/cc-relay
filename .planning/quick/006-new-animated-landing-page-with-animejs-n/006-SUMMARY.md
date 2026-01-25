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

# Quick Task 006: High-Speed Network Visualization Landing Page

Completely redesigned landing page with gorgeous high-speed network animation showing requests flowing from Claude Code through CC-Relay to 6 providers with comet-like particle trails.

## Key Features

### High-Speed Network Visualization
- **15 concurrent animated packets** with comet-like trails (8 particles each)
- Packets flow: Claude Code → CC-Relay → random provider (6 targets)
- **Variable speed**: 400-800ms per packet for organic traffic feel
- **150ms spawn interval**: Continuous high-speed stream
- Beautiful gradient bezier curves connecting nodes
- Provider nodes **flash on packet arrival**
- **Live request counter** showing cumulative traffic

### Animated CC-Relay Hub
- Pulsing gradient core (1.5s cycle)
- **3 concentric rotating rings** (8s, 12s, 20s - different directions)
- Ring opacity pulse (2s cycle with stagger)
- Intense glow filter for dramatic effect

### Provider Nodes (6 total)
- Anthropic, Z.AI, Ollama, AWS Bedrock, Azure, Vertex AI
- Each with logo, name, and description
- Subtle stroke-width animation on all nodes
- Drop shadow filter for depth

### Visual Polish
- Dark gradient background with subtle grid lines
- Beautiful color palette: pink, purple, indigo, blue, cyan, teal
- Intense glow filters on packets and trails
- Responsive design with mobile adjustments

## Technical Implementation

### AnimeJS Timeline API
```javascript
// Packet animation phases:
1. Main path (40% duration) - Claude Code to CC-Relay
2. Brief pause (50ms) - processing at relay
3. Provider path (50% duration) - CC-Relay to target
4. Fade out (100ms) - burst effect at destination
```

### SVG Structure
- Cubic bezier paths for smooth curves
- Dynamic packet/trail creation via JS
- Path length calculation with `getTotalLength()`
- Trail particles follow leader with progress offset

### Performance
- Packets and trails removed after animation
- Staggered initial burst prevents frame drop
- CSS filters optimized with explicit bounds

## Commits

| Commit | Description |
|--------|-------------|
| e9b6006 | feat(quick-006): add animated network visualization to landing page |
| 96bc5e9 | feat(landing): high-speed network visualization with AnimeJS |

## Files Changed

- `docs-site/content/en/_index.md` - Complete rewrite with inline styles and script
- `docs-site/static/logos/claude-code.svg` - Claude Code logo from dashboard-icons
