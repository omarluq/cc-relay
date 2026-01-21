---
title: CC-Relay
layout: hextra-home
---

<div class="landing-page">

<div class="custom-hero">
  <h1 class="hero-title">CC-Relay</h1>
  <p class="hero-subtitle">
    Redefining the Claude Code multi-model story
  </p>
  <div class="hero-buttons">
    <a href="/docs/getting-started/" class="hero-button hero-button-primary">Get Started</a>
    <a href="https://github.com/omarluq/cc-relay" class="hero-button hero-button-secondary">
      <svg class="github-icon" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
      </svg>
      GitHub
    </a>
  </div>
</div>

<div class="mt-6 mb-6">
{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Multi-Provider Support"
    subtitle="Connect to Anthropic, Z.AI, Ollama, AWS Bedrock, Azure, and Vertex AI from a single endpoint"
  >}}
  {{< hextra/feature-card
    title="Rate Limit Pooling"
    subtitle="Intelligently distribute requests across multiple API keys to maximize throughput"
  >}}
  {{< hextra/feature-card
    title="Cost Optimization"
    subtitle="Route requests based on cost, latency, or model availability for optimal efficiency"
  >}}
  {{< hextra/feature-card
    title="Automatic Failover"
    subtitle="Circuit breaker with health tracking ensures high availability across providers"
  >}}
  {{< hextra/feature-card
    title="Real-time Monitoring"
    subtitle="TUI dashboard with live stats, provider health, and request logging"
  >}}
  {{< hextra/feature-card
    title="Hot Reload Config"
    subtitle="Update provider settings and routing strategies without restart"
  >}}
{{< /hextra/feature-grid >}}
</div>

<div class="info-box">
  <div class="info-box-title">
    <span class="info-icon">⚡</span>
    One Proxy, Unlimited Scale
  </div>
  <div class="info-box-content">
    <div class="feature-item">
      <span class="feature-icon">🎯</span>
      <div>
        <strong>Smart Routing</strong>
        <p>Shuffle, round-robin, failover, cost-based, latency-based, or model-based strategies</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🔑</span>
      <div>
        <strong>API Key Pools</strong>
        <p>Manage multiple keys per provider with RPM/TPM tracking</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">☁️</span>
      <div>
        <strong>Cloud Provider Support</strong>
        <p>Native integration with Bedrock, Azure Foundry, and Vertex AI</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">💚</span>
      <div>
        <strong>Health Tracking</strong>
        <p>Automatic circuit breaking and recovery for failed providers</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">📡</span>
      <div>
        <strong>SSE Streaming</strong>
        <p>Perfect compatibility with Claude Code's real-time streaming</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🎮</span>
      <div>
        <strong>Management API</strong>
        <p>gRPC interface for stats, config updates, and provider control</p>
      </div>
    </div>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">Quick Start</h2>

<div class="terminal-container">
  <div class="terminal-header">
    <div class="terminal-buttons">
      <span class="terminal-button close"></span>
      <span class="terminal-button minimize"></span>
      <span class="terminal-button maximize"></span>
    </div>
    <div class="terminal-title">Terminal — bash</div>
  </div>
  <div class="terminal-body">
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-1"># Install</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-2">go install github.com/omarluq/cc-relay@latest</span>
    </div>
    <div class="terminal-line terminal-output typing-3">
      <span class="terminal-success">✓ installed cc-relay@latest</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-4"># Initialize configuration</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-5">cc-relay config init</span>
    </div>
    <div class="terminal-line terminal-output typing-6">
      <span class="terminal-success">✓ Config created at ~/.config/cc-relay/config.yaml</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-7"># Configure Claude Code integration</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-8">cc-relay config cc init</span>
    </div>
    <div class="terminal-line terminal-output typing-9">
      <span class="terminal-success">✓ Claude Code configured to use cc-relay</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-10"># Run the server</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-11">cc-relay serve</span>
    </div>
    <div class="terminal-line terminal-output typing-12">
      <span class="terminal-info">→ Server started on http://localhost:8787</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-13"># Start using Claude Code</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-14">claude</span>
    </div>
    <div class="terminal-line terminal-output typing-15">
      <span class="terminal-success">✓ Connected via cc-relay</span>
      <span class="terminal-cursor"></span>
    </div>
  </div>
</div>
</div>

<div class="section-box">
  <h2 class="section-title">Architecture</h2>
  <p class="section-description">CC-Relay sits between your LLM client and multiple providers, intelligently routing requests based on your configured strategy</p>

<div class="architecture-diagram">
  <div class="arch-layer">
    <div class="arch-layer-title">Client Layer</div>
    <div class="arch-node arch-node-client">
      <div class="client-icon">🤖</div>
      <div class="client-text">
        <strong>Claude Code Client</strong><br/>
        <span style="font-size: 0.875rem; opacity: 0.9;">POST /v1/messages</span>
      </div>
    </div>
  </div>

  <div class="arch-connector">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title">Proxy Engine</div>
    <div class="arch-proxy">
      <div class="arch-proxy-component">🔐 Authentication</div>
      <div class="arch-proxy-component">🎯 Smart Router</div>
      <div class="arch-proxy-component">💚 Health Tracker</div>
      <div class="arch-proxy-component">🔑 API Key Pool</div>
    </div>
  </div>

  <div class="arch-connector">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title">Provider Layer</div>
    <div class="arch-providers">
      <div class="arch-provider anthropic">
        <img src="/logos/anthropic.svg" alt="Anthropic" class="arch-provider-logo" />
        <div class="arch-provider-name">Anthropic</div>
        <div class="arch-provider-desc">Claude Models</div>
      </div>
      <div class="arch-provider zai">
        <img src="/logos/zai.svg" alt="Z.AI" class="arch-provider-logo" />
        <div class="arch-provider-name">Z.AI</div>
        <div class="arch-provider-desc">GLM Models</div>
      </div>
      <div class="arch-provider ollama">
        <img src="/logos/ollama.svg" alt="Ollama" class="arch-provider-logo" />
        <div class="arch-provider-name">Ollama</div>
        <div class="arch-provider-desc">Local Models</div>
      </div>
      <div class="arch-provider bedrock">
        <img src="/logos/aws.svg" alt="AWS Bedrock" class="arch-provider-logo" />
        <div class="arch-provider-name">AWS Bedrock</div>
        <div class="arch-provider-desc">SigV4 Auth</div>
      </div>
      <div class="arch-provider azure">
        <img src="/logos/azure.svg" alt="Azure" class="arch-provider-logo" />
        <div class="arch-provider-name">Azure Foundry</div>
        <div class="arch-provider-desc">Deployments</div>
      </div>
      <div class="arch-provider vertex">
        <img src="/logos/gcp.svg" alt="Vertex AI" class="arch-provider-logo" />
        <div class="arch-provider-name">Vertex AI</div>
        <div class="arch-provider-desc">OAuth</div>
      </div>
    </div>
  </div>
</div>
</div>

<div class="section-box">
  <h2 class="section-title">Use Cases</h2>
  <p class="section-description">Power your development workflow with intelligent LLM routing</p>

  <div class="use-cases-grid">
    <div class="use-case-card">
      <div class="use-case-icon">🤖</div>
      <h3>Multi-Agent Systems</h3>
      <p>Build HA multi-agent orchestration with intelligent routing and failover</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">👥</div>
      <h3>Development Teams</h3>
      <p>Share API quota across multiple developers</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">🚀</div>
      <h3>CI/CD Pipelines</h3>
      <p>High-throughput testing with rate limit pooling</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">💰</div>
      <h3>Cost Optimization</h3>
      <p>Route to cheapest provider while maintaining quality</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">⚡</div>
      <h3>High Availability</h3>
      <p>Automatic failover ensures uptime</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">☁️</div>
      <h3>Multi-Cloud</h3>
      <p>Leverage Bedrock, Azure, and Vertex AI simultaneously</p>
    </div>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">Documentation</h2>
  <p class="section-description">Everything you need to get started and master CC-Relay</p>

  <div class="docs-grid">
    {{< hextra/feature-card
      title="Getting Started"
      subtitle="Installation, configuration, and first run"
      link="/docs/getting-started/"
    >}}
    {{< hextra/feature-card
      title="Configuration"
      subtitle="Provider setup, routing strategies, and advanced options"
      link="/docs/configuration/"
    >}}
    {{< hextra/feature-card
      title="Architecture"
      subtitle="System design, components, and API compatibility"
      link="/docs/architecture/"
    >}}
    {{< hextra/feature-card
      title="API Reference"
      subtitle="gRPC management API and REST endpoints"
      link="/docs/api/"
    >}}
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">Contributing</h2>
  <p class="section-description">CC-Relay is open source! Contributions are welcome</p>

  <div class="contributing-links">
    <a href="https://github.com/omarluq/cc-relay/issues" class="contrib-link">
      <span class="contrib-icon">🐛</span>
      <span>Report bugs</span>
    </a>
    <a href="https://github.com/omarluq/cc-relay/issues" class="contrib-link">
      <span class="contrib-icon">💡</span>
      <span>Request features</span>
    </a>
    <a href="https://github.com/omarluq/cc-relay/pulls" class="contrib-link">
      <span class="contrib-icon">🚀</span>
      <span>Submit PRs</span>
    </a>
  </div>

  <div class="license-box">
    <p>MIT License - see <a href="https://github.com/omarluq/cc-relay/blob/main/LICENSE">LICENSE</a> for details</p>
  </div>
</div>

<div class="custom-footer">
  <p class="footer-powered">Powered by <a href="https://gohugo.io" target="_blank" rel="noopener">Hugo</a></p>
  <p class="footer-copyright">© 2026 Omar Alani. All rights reserved.</p>
</div>

</div><!-- End .landing-page -->
