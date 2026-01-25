---
phase: quick
plan: 006
type: execute
wave: 1
depends_on: []
files_modified:
  - docs-site/content/en/_index.md
  - docs-site/assets/css/custom.css
  - docs-site/static/logos/claude-code.svg
autonomous: true

must_haves:
  truths:
    - "Landing page shows animated network visualization with packets moving from Claude Code to cc-relay to providers"
    - "AnimeJS drives smooth animations for request packet flow"
    - "All provider logos render correctly including Kimi AI and Moonshot AI"
    - "Page is smaller and more focused than current version"
  artifacts:
    - path: "docs-site/content/en/_index.md"
      provides: "New landing page with network visualization"
    - path: "docs-site/assets/css/custom.css"
      provides: "Styles for network animation and logo fixes"
    - path: "docs-site/static/logos/claude-code.svg"
      provides: "Claude Code logo from dashboard-icons"
  key_links:
    - from: "docs-site/content/en/_index.md"
      to: "https://cdn.jsdelivr.net/npm/animejs@3.2.2"
      via: "script tag"
      pattern: "animejs"
---

<objective>
Create a new, smaller landing page with an animated high-speed network map showing requests flowing from Claude Code through cc-relay to multiple providers.

Purpose: Visually demonstrate cc-relay's multi-provider routing capability with smooth AnimeJS animations
Output: Refreshed landing page with network visualization, fixed logos
</objective>

<execution_context>
@./.claude/get-shit-done/workflows/execute-plan.md
@./.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@docs-site/content/en/_index.md
@docs-site/assets/css/custom.css
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create new animated landing page with network visualization</name>
  <files>
    docs-site/content/en/_index.md
    docs-site/assets/css/custom.css
    docs-site/static/logos/claude-code.svg
  </files>
  <action>
1. Copy Claude Code logo from dashboard-icons to project:
   - Source: /tmp/dashboard-icons/svg/claude-ai.svg
   - Destination: docs-site/static/logos/claude-code.svg

2. Replace docs-site/content/en/_index.md with a smaller, focused landing page:

   Hero Section (minimal):
   - CC-Relay title with gradient animation
   - One-liner subtitle: "Multi-provider proxy for Claude Code"
   - Two buttons: Get Started, GitHub

   Network Visualization (main feature):
   - Create SVG-based network diagram showing:
     - Left: Claude Code node (using claude-code.svg logo)
     - Center: CC-Relay hub (pink/purple gradient circle)
     - Right: 6 provider nodes arranged in arc (Anthropic, Z.AI, Ollama, Bedrock, Azure, Vertex)
   - Animated packets (small circles) traveling along paths from Claude Code to cc-relay to random providers
   - Use AnimeJS via CDN: https://cdn.jsdelivr.net/npm/animejs@3.2.2/lib/anime.min.js
   - Animation: Continuous loop of packets spawning, traveling through relay, routing to different providers
   - Include inline script with anime.js animation code

   Quick Start (compact):
   - 4-line terminal snippet (install, init, serve, connect)

   Footer (minimal):
   - GitHub, social links

3. Update docs-site/assets/css/custom.css:
   - Add .network-visualization container styles
   - Add .network-node styles for Claude Code, CC-Relay hub, and providers
   - Add .network-path styles for connection lines
   - Add .packet styles for animated request dots
   - Fix logo rendering by removing the overly aggressive filter that breaks Kimi/Moonshot logos
   - Keep dark theme consistent with existing styles
   - Responsive: Stack nodes vertically on mobile

4. Logo fixes in CSS:
   - Current issue: `.arch-provider-logo` has `filter: brightness(0) saturate(100%) invert(...)` that turns all logos white
   - This breaks logos with embedded colors (kimi.svg, moonshot.svg)
   - Solution: Remove the filter, let logos use their original colors (they're already designed for dark backgrounds)
   - Or: Create a new class .network-provider-logo without the filter
  </action>
  <verify>
    - Run `cd /home/omarluq/sandbox/go/cc-relay/docs-site && hugo server --port 1414` and visit http://localhost:1414
    - Verify network visualization shows Claude Code -> CC-Relay -> Providers
    - Verify packets animate smoothly along paths
    - Verify Kimi AI and Moonshot AI logos render (in "Coming Soon" section if kept)
    - Verify page is smaller/more focused than before
  </verify>
  <done>
    - Landing page has animated network visualization with packets flowing through cc-relay
    - AnimeJS animations are smooth and continuous
    - All logos render correctly (no blank images)
    - Page is more compact and focused on the key value prop
  </done>
</task>

</tasks>

<verification>
- Network visualization shows Claude Code -> CC-Relay hub -> 6 provider nodes
- Animated packets flow continuously from source through relay to providers
- Kimi AI and Moonshot AI logos render (no blank boxes)
- Page loads AnimeJS from CDN and runs animations
- Mobile responsive: visualization adapts to smaller screens
</verification>

<success_criteria>
- Animated network map is the centerpiece of the landing page
- Packets visually demonstrate multi-provider routing
- All provider logos render correctly
- Page is smaller/cleaner than current version
- AnimeJS CDN loads and animations run smoothly
</success_criteria>

<output>
After completion, create `.planning/quick/006-new-animated-landing-page-with-animejs-n/006-SUMMARY.md`
</output>
