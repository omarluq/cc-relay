<p align="center">
  <img src="ai-gophers.svg" alt="goophers" width="500" height="500"/>
</p>

<h1 align="center">CC-Relay</h1>

<h4 align="center">
Boost Claude Code by routing to multiple Anthropic-compatible providers simultaneously
</h4>

<p align="center">  
  <a href="https://cc-relay.ai/"><img src="https://img.shields.io/badge/-cc--relay.ai-5e5086?style=flat&labelColor=24292e&logo=safari&logoColor=white" alt="Website"></a>
  <a href="https://cc-relay.ai/en/docs/"><img src="https://img.shields.io/badge/-Read%20the%20Docs-blue?style=flat&labelColor=24292e&logo=readthedocs&logoColor=white" alt="Documentation"></a>
  <a href="https://pkg.go.dev/github.com/omarluq/cc-relay"><img src="https://img.shields.io/badge/reference-007d9c?style=flat&labelColor=24292e&logo=go&logoColor=white" alt="Go Reference"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/-%3E%3D1.18-00ADD8?style=flat&labelColor=24292e&logo=go&logoColor=white" alt="Go Version"></a>
  <a href="https://goreportcard.com/report/github.com/omarluq/cc-relay"><img src="https://img.shields.io/badge/report-A%2B-00ADD8?style=flat&labelColor=24292e&logo=go&logoColor=white" alt="Go Report Card"></a>
  <a href="https://github.com/omarluq/cc-relay/releases"><img src="https://img.shields.io/badge/-Latest%20Release-28a745?style=flat&labelColor=24292e&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0id2hpdGUiPjxwYXRoIGQ9Ik0xMiAxNWw1LTUtMS40MS0xLjQxTDEzIDExLjE3VjRoLTJ2Ny4xN0w4LjQxIDguNTkgNyAxMGw1IDV6bTcgMnY0SDV2LTRIMy42OHY0LjMzYzAgLjczNC41OTYgMS4zMyAxLjMzIDEuMzNoMTMuOThjLjczNCAwIDEuMzMtLjU5NiAxLjMzLTEuMzNWMTdIMTl6Ii8+PC9zdmc+" alt="Download"></a>
  <br/>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat&labelColor=24292e&logo=opensourceinitiative&logoColor=white" alt="License: AGPL-3.0"></a>
  <a href="https://github.com/omarluq/cc-relay/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/omarluq/cc-relay/ci.yml?style=flat&labelColor=24292e&label=Tests&logo=github&logoColor=white" alt="Tests"></a>
  <a href="https://github.com/omarluq/cc-relay/releases"><img src="https://img.shields.io/github/v/release/omarluq/cc-relay?style=flat&labelColor=24292e&color=28a745&label=Version&logo=semver&logoColor=white" alt="Version"></a>
  <a href="https://codecov.io/gh/omarluq/cc-relay"><img src="https://img.shields.io/codecov/c/github/omarluq/cc-relay?style=flat&labelColor=24292e&logo=codecov&logoColor=white&token=YW23EDL5T5" alt="codecov"></a>
  <a href="https://github.com/omarluq/cc-relay"><img src="https://img.shields.io/badge/Maintained%3F-yes-28a745?style=flat&labelColor=24292e&logo=checkmarx&logoColor=white" alt="Maintained"></a>
  <a href="https://github.com/omarluq/cc-relay"><img src="https://img.shields.io/badge/Made%20with-Love-ff69b4?style=flat&labelColor=24292e&logo=githubsponsors&logoColor=white" alt="Made with Love"></a>
</p>

<h2>Why?</h2>

<p>
  Claude Code connects to one provider at a time. But what if you want to:
</p>

<p>
  <strong>🔑 Pool rate limits</strong> across multiple Anthropic API keys<br>
  <strong>💰 Save money</strong> by routing simple tasks to lighter models<br>
  <strong>🛡️ Never get stuck</strong> with automatic failover between providers<br>
  <strong>🏢 Use your company's Bedrock/Azure/Vertex</strong> alongside personal API keys
</p>

<p>
  <strong>cc-relay</strong> makes all of this possible.
</p>

<p>

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'primaryColor': '#6366f1', 'primaryTextColor': '#fff', 'primaryBorderColor': '#4f46e5', 'lineColor': '#94a3b8', 'secondaryColor': 'transparent', 'tertiaryColor': 'transparent', 'background': 'transparent', 'mainBkg': 'transparent', 'nodeBorder': '#4f46e5', 'clusterBkg': 'transparent', 'clusterBorder': 'transparent', 'titleColor': '#1e293b', 'edgeLabelBackground': 'transparent'}}}%%

flowchart LR
    CC["👾 Claude Code"]
    RELAY["⚡ CC-Relay"]

    subgraph providers[" "]
        direction TB
        ANT["🤖 Anthropic"]
        ZAI["🤖 Z.AI"]
        OLL["🦙 Ollama"]
        BED["🤖 AWS Bedrock"]
        AZU["🤖 Azure Foundry"]
        VTX["🤖 Vertex AI"]
    end

    CC --> RELAY
    RELAY --> ANT
    RELAY --> ZAI
    RELAY --> OLL
    RELAY --> BED
    RELAY --> AZU
    RELAY --> VTX

    style CC fill:#1e1e2e,stroke:#6366f1,stroke-width:2px,color:#fff
    style RELAY fill:#6366f1,stroke:#4f46e5,stroke-width:3px,color:#fff
    style ANT fill:#ff6b35,stroke:#e55a2b,stroke-width:2px,color:#fff
    style ZAI fill:#3b82f6,stroke:#2563eb,stroke-width:2px,color:#fff
    style OLL fill:#22c55e,stroke:#16a34a,stroke-width:2px,color:#fff
    style BED fill:#f59e0b,stroke:#d97706,stroke-width:2px,color:#fff
    style AZU fill:#0ea5e9,stroke:#0284c7,stroke-width:2px,color:#fff
    style VTX fill:#ef4444,stroke:#dc2626,stroke-width:2px,color:#fff
    style providers fill:transparent,stroke:transparent
```

</p>

## License

[AGPL-3.0](https://github.com/omarluq/cc-relay/blob/main/LICENSE) - © 2026 [Omar Alani](https://github.com/omarluq)

## Contributing

Contributions welcome! Please open an issue before submitting PRs.
