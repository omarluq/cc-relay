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

<div class="network-section">
  <div class="network-visualization" id="network-viz">
    <svg viewBox="0 0 1000 500" class="network-svg" preserveAspectRatio="xMidYMid meet">
      <defs>
        <!-- Gradients -->
        <linearGradient id="relay-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" style="stop-color:#ec4899"/>
          <stop offset="100%" style="stop-color:#8b5cf6"/>
        </linearGradient>
        <linearGradient id="claude-gradient" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" style="stop-color:#6366f1"/>
          <stop offset="100%" style="stop-color:#8b5cf6"/>
        </linearGradient>
        <radialGradient id="packet-glow" cx="50%" cy="50%" r="50%">
          <stop offset="0%" style="stop-color:#fff;stop-opacity:1"/>
          <stop offset="50%" style="stop-color:#ec4899;stop-opacity:0.8"/>
          <stop offset="100%" style="stop-color:#ec4899;stop-opacity:0"/>
        </radialGradient>

        <!-- Glow filters -->
        <filter id="glow" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="4" result="coloredBlur"/>
          <feMerge>
            <feMergeNode in="coloredBlur"/>
            <feMergeNode in="SourceGraphic"/>
          </feMerge>
        </filter>
        <filter id="glow-intense" x="-100%" y="-100%" width="300%" height="300%">
          <feGaussianBlur stdDeviation="8" result="coloredBlur"/>
          <feMerge>
            <feMergeNode in="coloredBlur"/>
            <feMergeNode in="coloredBlur"/>
            <feMergeNode in="SourceGraphic"/>
          </feMerge>
        </filter>
        <filter id="node-shadow">
          <feDropShadow dx="0" dy="4" stdDeviation="8" flood-color="#000" flood-opacity="0.3"/>
        </filter>

        <!-- Animated dash pattern for paths -->
        <pattern id="dash-pattern" patternUnits="userSpaceOnUse" width="20" height="1">
          <line x1="0" y1="0" x2="10" y2="0" stroke="#6366f1" stroke-width="2" opacity="0.5"/>
        </pattern>
      </defs>

      <!-- Background grid effect -->
      <g class="grid-bg" opacity="0.1">
        <line x1="0" y1="250" x2="1000" y2="250" stroke="#6366f1" stroke-width="1"/>
        <line x1="150" y1="0" x2="150" y2="500" stroke="#6366f1" stroke-width="1"/>
        <line x1="500" y1="0" x2="500" y2="500" stroke="#6366f1" stroke-width="1"/>
        <line x1="850" y1="0" x2="850" y2="500" stroke="#6366f1" stroke-width="1"/>
      </g>

      <!-- Connection paths - curved beziers -->
      <g class="paths-container">
        <!-- Main path: Claude Code to CC-Relay -->
        <path id="path-main" class="network-path" d="M 200 250 C 300 250, 400 250, 500 250"
              stroke="#6366f1" stroke-width="2" fill="none" opacity="0.3"/>

        <!-- Provider paths with beautiful curves -->
        <path id="path-0" class="network-path" d="M 500 250 C 600 250, 700 60, 850 60"
              stroke="#d946ef" stroke-width="2" fill="none" opacity="0.3"/>
        <path id="path-1" class="network-path" d="M 500 250 C 580 200, 700 130, 850 130"
              stroke="#8b5cf6" stroke-width="2" fill="none" opacity="0.3"/>
        <path id="path-2" class="network-path" d="M 500 250 C 600 240, 700 200, 850 200"
              stroke="#6366f1" stroke-width="2" fill="none" opacity="0.3"/>
        <path id="path-3" class="network-path" d="M 500 250 C 600 260, 700 300, 850 300"
              stroke="#3b82f6" stroke-width="2" fill="none" opacity="0.3"/>
        <path id="path-4" class="network-path" d="M 500 250 C 580 300, 700 370, 850 370"
              stroke="#0ea5e9" stroke-width="2" fill="none" opacity="0.3"/>
        <path id="path-5" class="network-path" d="M 500 250 C 600 250, 700 440, 850 440"
              stroke="#14b8a6" stroke-width="2" fill="none" opacity="0.3"/>
      </g>

      <!-- Claude Code Node -->
      <g class="network-node claude-node" filter="url(#node-shadow)">
        <rect x="80" y="200" width="140" height="100" rx="16" fill="rgba(30, 41, 59, 0.95)" stroke="url(#claude-gradient)" stroke-width="2"/>
        <image href="/logos/claude-code.svg" x="115" y="215" width="50" height="50" class="node-logo"/>
        <text x="150" y="285" text-anchor="middle" fill="#e2e8f0" font-size="13" font-weight="600">Claude Code</text>
      </g>

      <!-- CC-Relay Hub - Animated center -->
      <g class="relay-hub" filter="url(#glow)">
        <circle class="relay-ring relay-ring-3" cx="500" cy="250" r="70" fill="none" stroke="#ec4899" stroke-width="1" opacity="0.2"/>
        <circle class="relay-ring relay-ring-2" cx="500" cy="250" r="55" fill="none" stroke="#8b5cf6" stroke-width="1" opacity="0.3"/>
        <circle class="relay-ring relay-ring-1" cx="500" cy="250" r="40" fill="none" stroke="#6366f1" stroke-width="2" opacity="0.5"/>
        <circle class="relay-core" cx="500" cy="250" r="35" fill="url(#relay-gradient)"/>
        <text x="500" y="245" text-anchor="middle" fill="white" font-size="12" font-weight="700">CC</text>
        <text x="500" y="260" text-anchor="middle" fill="white" font-size="12" font-weight="700">Relay</text>
      </g>

      <!-- Provider Nodes -->
      <g class="provider-nodes">
        <!-- Anthropic -->
        <g class="network-node provider-node provider-0" filter="url(#node-shadow)" transform="translate(800, 30)">
          <rect x="0" y="0" width="150" height="60" rx="12" fill="rgba(30, 41, 59, 0.95)" stroke="#d946ef" stroke-width="2"/>
          <image href="/logos/anthropic.svg" x="12" y="12" width="36" height="36" class="provider-logo"/>
          <text x="60" y="28" text-anchor="start" fill="#e2e8f0" font-size="13" font-weight="600">Anthropic</text>
          <text x="60" y="44" text-anchor="start" fill="#94a3b8" font-size="10">Claude Models</text>
        </g>

        <!-- Z.AI -->
        <g class="network-node provider-node provider-1" filter="url(#node-shadow)" transform="translate(800, 100)">
          <rect x="0" y="0" width="150" height="60" rx="12" fill="rgba(30, 41, 59, 0.95)" stroke="#8b5cf6" stroke-width="2"/>
          <image href="/logos/zai.svg" x="12" y="12" width="36" height="36" class="provider-logo"/>
          <text x="60" y="28" text-anchor="start" fill="#e2e8f0" font-size="13" font-weight="600">Z.AI</text>
          <text x="60" y="44" text-anchor="start" fill="#94a3b8" font-size="10">GLM Models</text>
        </g>

        <!-- Ollama -->
        <g class="network-node provider-node provider-2" filter="url(#node-shadow)" transform="translate(800, 170)">
          <rect x="0" y="0" width="150" height="60" rx="12" fill="rgba(30, 41, 59, 0.95)" stroke="#6366f1" stroke-width="2"/>
          <image href="/logos/ollama.svg" x="12" y="12" width="36" height="36" class="provider-logo"/>
          <text x="60" y="28" text-anchor="start" fill="#e2e8f0" font-size="13" font-weight="600">Ollama</text>
          <text x="60" y="44" text-anchor="start" fill="#94a3b8" font-size="10">Local Models</text>
        </g>

        <!-- AWS Bedrock -->
        <g class="network-node provider-node provider-3" filter="url(#node-shadow)" transform="translate(800, 270)">
          <rect x="0" y="0" width="150" height="60" rx="12" fill="rgba(30, 41, 59, 0.95)" stroke="#3b82f6" stroke-width="2"/>
          <image href="/logos/aws.svg" x="12" y="12" width="36" height="36" class="provider-logo"/>
          <text x="60" y="28" text-anchor="start" fill="#e2e8f0" font-size="13" font-weight="600">Bedrock</text>
          <text x="60" y="44" text-anchor="start" fill="#94a3b8" font-size="10">AWS SigV4</text>
        </g>

        <!-- Azure -->
        <g class="network-node provider-node provider-4" filter="url(#node-shadow)" transform="translate(800, 340)">
          <rect x="0" y="0" width="150" height="60" rx="12" fill="rgba(30, 41, 59, 0.95)" stroke="#0ea5e9" stroke-width="2"/>
          <image href="/logos/azure.svg" x="12" y="12" width="36" height="36" class="provider-logo"/>
          <text x="60" y="28" text-anchor="start" fill="#e2e8f0" font-size="13" font-weight="600">Azure</text>
          <text x="60" y="44" text-anchor="start" fill="#94a3b8" font-size="10">Foundry</text>
        </g>

        <!-- Vertex AI -->
        <g class="network-node provider-node provider-5" filter="url(#node-shadow)" transform="translate(800, 410)">
          <rect x="0" y="0" width="150" height="60" rx="12" fill="rgba(30, 41, 59, 0.95)" stroke="#14b8a6" stroke-width="2"/>
          <image href="/logos/gcp.svg" x="12" y="12" width="36" height="36" class="provider-logo"/>
          <text x="60" y="28" text-anchor="start" fill="#e2e8f0" font-size="13" font-weight="600">Vertex AI</text>
          <text x="60" y="44" text-anchor="start" fill="#94a3b8" font-size="10">Google Cloud</text>
        </g>
      </g>

      <!-- Packet container - packets will be dynamically created -->
      <g class="packets-container" filter="url(#glow-intense)">
        <!-- Packets will be inserted here by JS -->
      </g>

      <!-- Trail container for comet-like effects -->
      <g class="trails-container">
        <!-- Trails will be inserted here by JS -->
      </g>
    </svg>

    <!-- Stats overlay -->
    <div class="network-stats">
      <div class="stat">
        <span class="stat-value" id="requests-count">0</span>
        <span class="stat-label">requests</span>
      </div>
    </div>
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

<!-- AnimeJS for high-speed network visualization -->
<script src="https://cdn.jsdelivr.net/npm/animejs@3.2.2/lib/anime.min.js"></script>
<script>
(function() {
  'use strict';

  // Configuration
  const CONFIG = {
    packetCount: 15,           // Number of concurrent packets
    minDuration: 400,          // Fastest packet (ms)
    maxDuration: 800,          // Slowest packet (ms)
    spawnInterval: 150,        // Time between new packets (ms)
    trailLength: 8,            // Number of trail particles
    colors: [
      '#ec4899', '#d946ef', '#a855f7', '#8b5cf6',
      '#6366f1', '#3b82f6', '#0ea5e9', '#14b8a6'
    ]
  };

  let requestCount = 0;
  let svg, packetsContainer, trailsContainer, mainPath, providerPaths;

  document.addEventListener('DOMContentLoaded', init);

  function init() {
    svg = document.querySelector('.network-svg');
    if (!svg) return;

    packetsContainer = svg.querySelector('.packets-container');
    trailsContainer = svg.querySelector('.trails-container');
    mainPath = document.getElementById('path-main');

    providerPaths = [];
    for (let i = 0; i <= 5; i++) {
      providerPaths.push(document.getElementById('path-' + i));
    }

    // Start animations
    animateRelayHub();
    animateProviderNodes();
    startPacketStream();
  }

  function animateRelayHub() {
    // Pulsing core
    anime({
      targets: '.relay-core',
      scale: [1, 1.1, 1],
      duration: 1500,
      easing: 'easeInOutSine',
      loop: true
    });

    // Rotating rings
    anime({
      targets: '.relay-ring-1',
      rotate: 360,
      duration: 8000,
      easing: 'linear',
      loop: true
    });

    anime({
      targets: '.relay-ring-2',
      rotate: -360,
      duration: 12000,
      easing: 'linear',
      loop: true
    });

    anime({
      targets: '.relay-ring-3',
      rotate: 360,
      duration: 20000,
      easing: 'linear',
      loop: true
    });

    // Ring pulse
    anime({
      targets: '.relay-ring',
      opacity: [0.2, 0.5, 0.2],
      duration: 2000,
      easing: 'easeInOutQuad',
      loop: true,
      delay: anime.stagger(200)
    });
  }

  function animateProviderNodes() {
    // Subtle hover effect on provider nodes
    anime({
      targets: '.provider-node rect',
      strokeWidth: [2, 2.5, 2],
      duration: 2000,
      easing: 'easeInOutSine',
      loop: true,
      delay: anime.stagger(300)
    });
  }

  function startPacketStream() {
    // Launch initial burst
    for (let i = 0; i < CONFIG.packetCount; i++) {
      setTimeout(function() {
        launchPacket();
      }, i * 80);
    }

    // Continuous stream
    setInterval(launchPacket, CONFIG.spawnInterval);
  }

  function launchPacket() {
    const providerIndex = Math.floor(Math.random() * 6);
    const color = CONFIG.colors[Math.floor(Math.random() * CONFIG.colors.length)];
    const duration = CONFIG.minDuration + Math.random() * (CONFIG.maxDuration - CONFIG.minDuration);

    // Create packet element
    const packet = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    packet.setAttribute('r', '4');
    packet.setAttribute('fill', color);
    packet.classList.add('packet');
    packetsContainer.appendChild(packet);

    // Create trail elements
    const trails = [];
    for (let i = 0; i < CONFIG.trailLength; i++) {
      const trail = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
      trail.setAttribute('r', String(3 - (i * 0.3)));
      trail.setAttribute('fill', color);
      trail.setAttribute('opacity', String(0.6 - (i * 0.07)));
      trail.classList.add('trail');
      trailsContainer.appendChild(trail);
      trails.push(trail);
    }

    // Get path data
    const mainPathLength = mainPath.getTotalLength();
    const providerPath = providerPaths[providerIndex];
    const providerPathLength = providerPath.getTotalLength();

    // Phase 1: Claude Code to CC-Relay
    const timeline = anime.timeline({
      easing: 'easeInOutQuart',
      complete: function() {
        // Cleanup
        packet.remove();
        trails.forEach(function(t) { t.remove(); });

        // Flash provider node
        flashProvider(providerIndex);

        // Update counter
        requestCount++;
        const counter = document.getElementById('requests-count');
        if (counter) counter.textContent = requestCount;
      }
    });

    // Animate through main path
    timeline.add({
      duration: duration * 0.4,
      update: function(anim) {
        const progress = anim.progress / 100;
        const point = mainPath.getPointAtLength(progress * mainPathLength);
        packet.setAttribute('cx', point.x);
        packet.setAttribute('cy', point.y);

        // Update trails with delay
        trails.forEach(function(trail, i) {
          const trailProgress = Math.max(0, progress - (i + 1) * 0.03);
          const trailPoint = mainPath.getPointAtLength(trailProgress * mainPathLength);
          trail.setAttribute('cx', trailPoint.x);
          trail.setAttribute('cy', trailPoint.y);
        });
      }
    });

    // Brief pause at relay (processing)
    timeline.add({
      duration: 50,
      update: function() {
        const point = mainPath.getPointAtLength(mainPathLength);
        packet.setAttribute('cx', point.x);
        packet.setAttribute('cy', point.y);
      }
    });

    // Animate through provider path
    timeline.add({
      duration: duration * 0.5,
      update: function(anim) {
        const progress = anim.progress / 100;
        const point = providerPath.getPointAtLength(progress * providerPathLength);
        packet.setAttribute('cx', point.x);
        packet.setAttribute('cy', point.y);

        // Update trails
        trails.forEach(function(trail, i) {
          const trailProgress = Math.max(0, progress - (i + 1) * 0.04);
          if (trailProgress > 0) {
            const trailPoint = providerPath.getPointAtLength(trailProgress * providerPathLength);
            trail.setAttribute('cx', trailPoint.x);
            trail.setAttribute('cy', trailPoint.y);
          }
        });
      }
    });

    // Fade out at destination
    timeline.add({
      targets: [packet].concat(trails),
      opacity: 0,
      scale: 1.5,
      duration: 100,
      easing: 'easeOutQuad'
    });
  }

  function flashProvider(index) {
    const provider = document.querySelector('.provider-' + index + ' rect');
    if (!provider) return;

    anime({
      targets: provider,
      strokeWidth: [2, 4, 2],
      duration: 300,
      easing: 'easeOutQuad'
    });
  }
})();
</script>

<style>
/* Network visualization styles */
.network-section {
  margin: 2rem 0;
}

.network-visualization {
  position: relative;
  width: 100%;
  max-width: 1000px;
  margin: 0 auto;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.95) 0%, rgba(30, 41, 59, 0.9) 100%);
  border-radius: 20px;
  padding: 1rem;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 0 1px rgba(99, 102, 241, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
  overflow: hidden;
}

.network-svg {
  width: 100%;
  height: auto;
  display: block;
}

.network-path {
  stroke-linecap: round;
}

.packet {
  filter: url(#glow-intense);
}

.trail {
  pointer-events: none;
}

.relay-hub {
  transform-origin: 500px 250px;
}

.relay-ring {
  transform-origin: 500px 250px;
}

.provider-logo {
  /* No filter - let logos use their natural colors */
}

.network-stats {
  position: absolute;
  bottom: 1.5rem;
  left: 1.5rem;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(99, 102, 241, 0.3);
  border-radius: 12px;
  padding: 0.75rem 1rem;
  backdrop-filter: blur(8px);
}

.stat {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
}

.stat-value {
  font-size: 1.5rem;
  font-weight: 700;
  color: #ec4899;
  font-variant-numeric: tabular-nums;
}

.stat-label {
  font-size: 0.75rem;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .network-visualization {
    padding: 0.5rem;
    border-radius: 12px;
  }

  .network-stats {
    bottom: 0.75rem;
    left: 0.75rem;
    padding: 0.5rem 0.75rem;
  }

  .stat-value {
    font-size: 1.25rem;
  }
}
</style>
