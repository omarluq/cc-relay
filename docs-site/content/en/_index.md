---
title: CC-Relay
layout: hextra-home
---

<div class="landing-page">

<div class="custom-hero">
  <h1 class="hero-title">CC-Relay</h1>
  <p class="hero-subtitle">
    Multi-provider proxy for Claude Code
  </p>
  <div class="hero-buttons">
    <a href="docs/getting-started/" class="hero-button hero-button-primary">Get Started</a>
    <a href="https://github.com/omarluq/cc-relay" class="hero-button hero-button-secondary">
      <svg class="github-icon" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
      </svg>
      GitHub
    </a>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">How It Works</h2>
  <p class="section-description">Route requests from Claude Code through cc-relay to any provider</p>

  <div class="network-visualization" id="network-viz">
    <svg viewBox="0 0 900 400" class="network-svg">
      <defs>
        <linearGradient id="relay-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" style="stop-color:#ec4899"/>
          <stop offset="100%" style="stop-color:#8b5cf6"/>
        </linearGradient>
        <linearGradient id="path-gradient" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" style="stop-color:#6366f1;stop-opacity:0.3"/>
          <stop offset="50%" style="stop-color:#ec4899;stop-opacity:0.6"/>
          <stop offset="100%" style="stop-color:#8b5cf6;stop-opacity:0.3"/>
        </linearGradient>
        <filter id="glow">
          <feGaussianBlur stdDeviation="3" result="coloredBlur"/>
          <feMerge>
            <feMergeNode in="coloredBlur"/>
            <feMergeNode in="SourceGraphic"/>
          </feMerge>
        </filter>
      </defs>

      <!-- Connection paths from Claude Code to CC-Relay -->
      <path class="network-path path-main" d="M 150 200 Q 300 200 450 200" stroke="url(#path-gradient)" stroke-width="3" fill="none"/>

      <!-- Connection paths from CC-Relay to providers (arc arrangement) -->
      <path class="network-path path-1" d="M 450 200 Q 600 80 750 80" stroke="url(#path-gradient)" stroke-width="2" fill="none"/>
      <path class="network-path path-2" d="M 450 200 Q 600 140 750 140" stroke="url(#path-gradient)" stroke-width="2" fill="none"/>
      <path class="network-path path-3" d="M 450 200 Q 600 200 750 200" stroke="url(#path-gradient)" stroke-width="2" fill="none"/>
      <path class="network-path path-4" d="M 450 200 Q 600 260 750 260" stroke="url(#path-gradient)" stroke-width="2" fill="none"/>
      <path class="network-path path-5" d="M 450 200 Q 600 320 750 320" stroke="url(#path-gradient)" stroke-width="2" fill="none"/>

      <!-- Claude Code Node -->
      <g class="network-node claude-node" transform="translate(100, 160)">
        <rect x="0" y="0" width="100" height="80" rx="12" fill="rgba(30, 41, 59, 0.9)" stroke="#6366f1" stroke-width="2"/>
        <image href="/logos/claude-code.svg" x="25" y="10" width="50" height="40" class="node-logo"/>
        <text x="50" y="65" text-anchor="middle" fill="#e2e8f0" font-size="11" font-weight="600">Claude Code</text>
      </g>

      <!-- CC-Relay Hub -->
      <g class="network-node relay-hub" transform="translate(400, 150)">
        <circle cx="50" cy="50" r="50" fill="url(#relay-gradient)" filter="url(#glow)"/>
        <text x="50" y="45" text-anchor="middle" fill="white" font-size="14" font-weight="700">CC</text>
        <text x="50" y="62" text-anchor="middle" fill="white" font-size="14" font-weight="700">Relay</text>
      </g>

      <!-- Provider Nodes -->
      <g class="network-node provider-node" transform="translate(700, 50)">
        <rect x="0" y="0" width="100" height="60" rx="10" fill="rgba(30, 41, 59, 0.9)" stroke="#10b981" stroke-width="2"/>
        <image href="/logos/anthropic.svg" x="10" y="10" width="30" height="30" class="node-logo provider-logo-svg"/>
        <text x="55" y="30" text-anchor="start" fill="#e2e8f0" font-size="11" font-weight="600">Anthropic</text>
      </g>

      <g class="network-node provider-node" transform="translate(700, 110)">
        <rect x="0" y="0" width="100" height="60" rx="10" fill="rgba(30, 41, 59, 0.9)" stroke="#8b5cf6" stroke-width="2"/>
        <image href="/logos/zai.svg" x="10" y="10" width="30" height="30" class="node-logo provider-logo-svg"/>
        <text x="55" y="30" text-anchor="start" fill="#e2e8f0" font-size="11" font-weight="600">Z.AI</text>
      </g>

      <g class="network-node provider-node" transform="translate(700, 170)">
        <rect x="0" y="0" width="100" height="60" rx="10" fill="rgba(30, 41, 59, 0.9)" stroke="#3b82f6" stroke-width="2"/>
        <image href="/logos/ollama.svg" x="10" y="10" width="30" height="30" class="node-logo provider-logo-svg"/>
        <text x="55" y="30" text-anchor="start" fill="#e2e8f0" font-size="11" font-weight="600">Ollama</text>
      </g>

      <g class="network-node provider-node" transform="translate(700, 230)">
        <rect x="0" y="0" width="100" height="60" rx="10" fill="rgba(30, 41, 59, 0.9)" stroke="#ff9900" stroke-width="2"/>
        <image href="/logos/aws.svg" x="10" y="10" width="30" height="30" class="node-logo provider-logo-svg"/>
        <text x="55" y="30" text-anchor="start" fill="#e2e8f0" font-size="11" font-weight="600">Bedrock</text>
      </g>

      <g class="network-node provider-node" transform="translate(700, 290)">
        <rect x="0" y="0" width="100" height="60" rx="10" fill="rgba(30, 41, 59, 0.9)" stroke="#0078d4" stroke-width="2"/>
        <image href="/logos/azure.svg" x="10" y="10" width="30" height="30" class="node-logo provider-logo-svg"/>
        <text x="55" y="30" text-anchor="start" fill="#e2e8f0" font-size="11" font-weight="600">Azure</text>
      </g>

      <!-- Animated Packets (will be animated with AnimeJS) -->
      <circle class="packet packet-1" cx="150" cy="200" r="6" fill="#ec4899" filter="url(#glow)"/>
      <circle class="packet packet-2" cx="150" cy="200" r="6" fill="#6366f1" filter="url(#glow)"/>
      <circle class="packet packet-3" cx="150" cy="200" r="6" fill="#8b5cf6" filter="url(#glow)"/>
      <circle class="packet packet-4" cx="150" cy="200" r="6" fill="#10b981" filter="url(#glow)"/>
      <circle class="packet packet-5" cx="150" cy="200" r="6" fill="#f97316" filter="url(#glow)"/>
    </svg>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">Quick Start</h2>

<div class="terminal-container terminal-compact">
  <div class="terminal-header">
    <div class="terminal-buttons">
      <span class="terminal-button close"></span>
      <span class="terminal-button minimize"></span>
      <span class="terminal-button maximize"></span>
    </div>
    <div class="terminal-title">Terminal</div>
  </div>
  <div class="terminal-body">
    <div class="terminal-line show-immediately">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command">go install github.com/omarluq/cc-relay@latest</span>
    </div>
    <div class="terminal-line show-immediately">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command">cc-relay config init && cc-relay config cc init</span>
    </div>
    <div class="terminal-line show-immediately">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command">cc-relay serve</span>
    </div>
    <div class="terminal-line terminal-output show-immediately">
      <span class="terminal-info">Server started on http://localhost:8787</span>
    </div>
  </div>
</div>
</div>

<div class="section-box">
  <h2 class="section-title">Features</h2>
  <div class="features-compact">
    <div class="feature-compact">
      <span class="feature-icon-compact">6</span>
      <div><strong>Providers</strong><br/><span>Anthropic, Z.AI, Ollama, Bedrock, Azure, Vertex</span></div>
    </div>
    <div class="feature-compact">
      <span class="feature-icon-compact">N</span>
      <div><strong>API Keys</strong><br/><span>Pool multiple keys for higher throughput</span></div>
    </div>
    <div class="feature-compact">
      <span class="feature-icon-compact">5</span>
      <div><strong>Routing Strategies</strong><br/><span>Failover, round-robin, weighted, shuffle, cost-based</span></div>
    </div>
    <div class="feature-compact">
      <span class="feature-icon-compact">%</span>
      <div><strong>100% Compatible</strong><br/><span>Drop-in Anthropic API replacement</span></div>
    </div>
  </div>
</div>

<div class="custom-footer">
  <div class="footer-social">
    <a href="https://github.com/omarluq" target="_blank" rel="noopener" aria-label="GitHub">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
      </svg>
    </a>
    <a href="https://linkedin.com/in/omarluq" target="_blank" rel="noopener" aria-label="LinkedIn">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
        <path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433c-1.144 0-2.063-.926-2.063-2.065 0-1.138.92-2.063 2.063-2.063 1.14 0 2.064.925 2.064 2.063 0 1.139-.925 2.065-2.064 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/>
      </svg>
    </a>
    <a href="https://bsky.app/profile/omarluq.bsky.social" target="_blank" rel="noopener" aria-label="Bluesky">
      <svg width="24" height="24" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 10.8c-1.087-2.114-4.046-6.053-6.798-7.995C2.566.944 1.561 1.266.902 1.565.139 1.908 0 3.08 0 3.768c0 .69.378 5.65.624 6.479.815 2.736 3.713 3.66 6.383 3.364.136-.02.275-.038.415-.056-.138.022-.276.04-.415.056-3.912.58-7.387 2.005-2.83 7.078 5.013 5.19 6.87-1.113 7.823-4.308.953 3.195 2.05 9.271 7.733 4.308 4.267-4.308 1.172-6.498-2.74-7.078a8.741 8.741 0 0 1-.415-.056c.14.018.279.036.415.056 2.67.297 5.568-.628 6.383-3.364.246-.828.624-5.79.624-6.478 0-.69-.139-1.861-.902-2.206-.659-.298-1.664-.62-4.3 1.24C16.046 4.748 13.087 8.687 12 10.8z"/>
      </svg>
    </a>
  </div>
  <p class="footer-powered">Powered by <a href="https://gohugo.io" target="_blank" rel="noopener">Hugo</a> | <a href="https://github.com/omarluq/cc-relay/blob/main/LICENSE">AGPL-3.0</a></p>
</div>

</div><!-- End .landing-page -->

<!-- AnimeJS for network visualization -->
<script src="https://cdn.jsdelivr.net/npm/animejs@3.2.2/lib/anime.min.js"></script>
<script>
document.addEventListener('DOMContentLoaded', function() {
  // Define the paths for packets to follow
  const mainPath = document.querySelector('.path-main');
  const providerPaths = [
    document.querySelector('.path-1'),
    document.querySelector('.path-2'),
    document.querySelector('.path-3'),
    document.querySelector('.path-4'),
    document.querySelector('.path-5')
  ];

  // Animate packets continuously
  function animatePacket(packet, pathIndex) {
    const mainPathLength = mainPath.getTotalLength();
    const providerPath = providerPaths[pathIndex];
    const providerPathLength = providerPath.getTotalLength();

    // Reset packet position
    packet.setAttribute('cx', '150');
    packet.setAttribute('cy', '200');

    // Animate along main path first
    anime({
      targets: packet,
      duration: 800,
      easing: 'easeInOutQuad',
      update: function(anim) {
        const progress = anim.progress / 100;
        const point = mainPath.getPointAtLength(progress * mainPathLength);
        packet.setAttribute('cx', point.x);
        packet.setAttribute('cy', point.y);
      },
      complete: function() {
        // Then animate along provider path
        anime({
          targets: packet,
          duration: 600,
          easing: 'easeOutQuad',
          update: function(anim) {
            const progress = anim.progress / 100;
            const point = providerPath.getPointAtLength(progress * providerPathLength);
            packet.setAttribute('cx', point.x);
            packet.setAttribute('cy', point.y);
          },
          complete: function() {
            // Fade out
            anime({
              targets: packet,
              opacity: [1, 0],
              duration: 200,
              easing: 'easeOutQuad',
              complete: function() {
                // Reset and restart with random provider
                packet.style.opacity = 1;
                const newPathIndex = Math.floor(Math.random() * 5);
                setTimeout(function() {
                  animatePacket(packet, newPathIndex);
                }, Math.random() * 500);
              }
            });
          }
        });
      }
    });
  }

  // Start animations with staggered timing
  const packets = document.querySelectorAll('.packet');
  packets.forEach(function(packet, i) {
    setTimeout(function() {
      animatePacket(packet, i % 5);
    }, i * 400);
  });

  // Pulse the relay hub
  anime({
    targets: '.relay-hub circle',
    scale: [1, 1.05, 1],
    duration: 2000,
    easing: 'easeInOutSine',
    loop: true
  });
});
</script>
