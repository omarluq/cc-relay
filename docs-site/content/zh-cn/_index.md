---
title: CC-Relay
layout: hextra-home
---

<div class="landing-page">

<div class="custom-hero">
  <h1 class="hero-title">CC-Relay</h1>
  <p class="hero-subtitle">
    重新定义 Claude Code 多模型体验
  </p>
  <div class="hero-buttons">
    <a href="docs/getting-started/" class="hero-button hero-button-primary">快速开始</a>
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
    title="多供应商支持"
    subtitle="通过单一端点连接 Anthropic 和 Z.AI（更多供应商即将支持）"
  >}}
  {{< hextra/feature-card
    title="SSE 流式传输"
    subtitle="完整的服务器推送事件支持，与 Claude Code 完美兼容"
  >}}
  {{< hextra/feature-card
    title="多 API 密钥"
    subtitle="每个供应商支持多个 API 密钥池，提升吞吐量"
  >}}
  {{< hextra/feature-card
    title="灵活认证"
    subtitle="支持 API 密钥和 Bearer Token，适配 Claude Code 订阅用户"
  >}}
  {{< hextra/feature-card
    title="Claude Code 集成"
    subtitle="一条命令完成配置，内置配置管理功能"
  >}}
  {{< hextra/feature-card
    title="Anthropic API 兼容"
    subtitle="即插即用，无需修改客户端代码"
  >}}
{{< /hextra/feature-grid >}}
</div>

<div class="info-box">
  <div class="info-box-title">
    <span class="info-icon">⚡</span>
    当前功能
  </div>
  <div class="info-box-content">
    <div class="feature-item">
      <span class="feature-icon">🔑</span>
      <div>
        <strong>多 API 密钥</strong>
        <p>每个供应商支持多密钥池，提升吞吐量</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🔐</span>
      <div>
        <strong>多认证方式</strong>
        <p>API 密钥和 Bearer Token 认证，灵活访问</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">📡</span>
      <div>
        <strong>SSE 流式传输</strong>
        <p>与 Claude Code 实时流式传输完美兼容</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🎛️</span>
      <div>
        <strong>调试日志</strong>
        <p>详细的请求/响应日志，便于故障排查</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">⚙️</span>
      <div>
        <strong>环境变量</strong>
        <p>在 YAML 中使用 ${VAR} 语法进行安全配置</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🚀</span>
      <div>
        <strong>快速配置</strong>
        <p>使用 cc-relay config cc init 一条命令完成 Claude Code 集成</p>
      </div>
    </div>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">快速开始</h2>

<div class="terminal-container">
  <div class="terminal-header">
    <div class="terminal-buttons">
      <span class="terminal-button close"></span>
      <span class="terminal-button minimize"></span>
      <span class="terminal-button maximize"></span>
    </div>
    <div class="terminal-title">终端 — bash</div>
  </div>
  <div class="terminal-body">
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-1"># 安装</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-2">go install github.com/omarluq/cc-relay@latest</span>
    </div>
    <div class="terminal-line terminal-output typing-3">
      <span class="terminal-success">✓ 已安装 cc-relay@latest</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-4"># 初始化配置</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-5">cc-relay config init</span>
    </div>
    <div class="terminal-line terminal-output typing-6">
      <span class="terminal-success">✓ 配置文件已创建于 ~/.config/cc-relay/config.yaml</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-7"># 配置 Claude Code 集成</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-8">cc-relay config cc init</span>
    </div>
    <div class="terminal-line terminal-output typing-9">
      <span class="terminal-success">✓ Claude Code 已配置使用 cc-relay</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-10"># 运行服务器</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-11">cc-relay serve</span>
    </div>
    <div class="terminal-line terminal-output typing-12">
      <span class="terminal-info">→ 服务器已启动于 http://localhost:8787</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-13"># 开始使用 Claude Code</span>
    </div>
    <div class="terminal-line">
      <span class="terminal-prompt">$</span>
      <span class="terminal-command typing-14">claude</span>
    </div>
    <div class="terminal-line terminal-output typing-15">
      <span class="terminal-success">✓ 已通过 cc-relay 连接</span>
      <span class="terminal-cursor"></span>
    </div>
  </div>
</div>
</div>

<div class="section-box">
  <h2 class="section-title">架构</h2>
  <p class="section-description">CC-Relay 位于您的 LLM 客户端和后端供应商之间，完全兼容 Anthropic API 进行请求代理</p>

<div class="architecture-diagram">
  <div class="arch-layer">
    <div class="arch-layer-title">客户端层</div>
    <div class="arch-node arch-node-client">
      <div class="client-icon">🤖</div>
      <div class="client-text">
        <strong>Claude Code 客户端</strong><br/>
        <span style="font-size: 0.875rem; opacity: 0.9;">POST /v1/messages</span>
      </div>
    </div>
  </div>

  <div class="arch-connector">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title">代理引擎</div>
    <div class="arch-proxy">
      <div class="arch-proxy-component">🔐 认证</div>
      <div class="arch-proxy-component">📝 请求日志</div>
      <div class="arch-proxy-component">📡 SSE 流式</div>
      <div class="arch-proxy-component">🔑 API 密钥管理</div>
    </div>
  </div>

  <div class="arch-connector">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title">供应商层（已实现）</div>
    <div class="arch-providers">
      <div class="arch-provider anthropic">
        <img src="/logos/anthropic.svg" alt="Anthropic" class="arch-provider-logo" />
        <div class="arch-provider-name">Anthropic</div>
        <div class="arch-provider-desc">Claude 模型</div>
      </div>
      <div class="arch-provider zai">
        <img src="/logos/zai.svg" alt="Z.AI" class="arch-provider-logo" />
        <div class="arch-provider-name">Z.AI</div>
        <div class="arch-provider-desc">GLM 模型</div>
      </div>
    </div>
  </div>

  <div class="arch-connector" style="margin-top: 1rem;">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title" style="opacity: 0.7;">即将支持</div>
    <div class="arch-providers" style="opacity: 0.6;">
      <div class="arch-provider ollama">
        <img src="/logos/ollama.svg" alt="Ollama" class="arch-provider-logo" />
        <div class="arch-provider-name">Ollama</div>
        <div class="arch-provider-desc">本地模型</div>
      </div>
      <div class="arch-provider bedrock">
        <img src="/logos/aws.svg" alt="AWS Bedrock" class="arch-provider-logo" />
        <div class="arch-provider-name">AWS Bedrock</div>
        <div class="arch-provider-desc">SigV4 认证</div>
      </div>
      <div class="arch-provider azure">
        <img src="/logos/azure.svg" alt="Azure" class="arch-provider-logo" />
        <div class="arch-provider-name">Azure Foundry</div>
        <div class="arch-provider-desc">部署</div>
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
  <h2 class="section-title">使用场景</h2>
  <p class="section-description">使用 CC-Relay 强化您的开发工作流</p>

  <div class="use-cases-grid">
    <div class="use-case-card">
      <div class="use-case-icon">🔄</div>
      <h3>供应商灵活切换</h3>
      <p>在 Anthropic 和 Z.AI 之间切换，无需修改客户端代码</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">👥</div>
      <h3>开发团队</h3>
      <p>通过密钥池在多个开发者之间共享 API 配额</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">🔑</div>
      <h3>API 密钥管理</h3>
      <p>集中管理和轮换 API 密钥，无需更新客户端</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">💰</div>
      <h3>成本对比</h3>
      <p>测试 Z.AI 的 GLM 模型作为低成本替代方案</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">🔐</div>
      <h3>订阅透传</h3>
      <p>将 Claude Code 订阅用户路由通过您的代理</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">🐛</div>
      <h3>请求调试</h3>
      <p>记录和检查 API 请求，便于故障排查</p>
    </div>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">文档</h2>
  <p class="section-description">您开始使用和掌握 CC-Relay 所需的一切</p>

  <div class="docs-grid">
    {{< hextra/feature-card
      title="快速开始"
      subtitle="安装、配置和首次运行"
      link="/cc-relay/zh/docs/getting-started/"
    >}}
    {{< hextra/feature-card
      title="配置"
      subtitle="供应商设置、路由策略和高级选项"
      link="/cc-relay/zh/docs/configuration/"
    >}}
    {{< hextra/feature-card
      title="架构"
      subtitle="系统设计、组件和 API 兼容性"
      link="/cc-relay/zh/docs/architecture/"
    >}}
    {{< hextra/feature-card
      title="API 参考"
      subtitle="HTTP 端点、流式传输和客户端示例"
      link="/cc-relay/zh/docs/api/"
    >}}
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">贡献</h2>
  <p class="section-description">CC-Relay 是开源项目！欢迎贡献</p>

  <div class="contributing-links">
    <a href="https://github.com/omarluq/cc-relay/issues" class="contrib-link">
      <span class="contrib-icon">🐛</span>
      <span>报告问题</span>
    </a>
    <a href="https://github.com/omarluq/cc-relay/issues" class="contrib-link">
      <span class="contrib-icon">💡</span>
      <span>功能建议</span>
    </a>
    <a href="https://github.com/omarluq/cc-relay/pulls" class="contrib-link">
      <span class="contrib-icon">🚀</span>
      <span>提交 PR</span>
    </a>
  </div>

  <div class="license-box">
    <p>AGPL 3 许可证 - 详见 <a href="https://github.com/omarluq/cc-relay/blob/main/LICENSE">LICENSE</a></p>
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
  <p class="footer-powered">由 <a href="https://gohugo.io" target="_blank" rel="noopener">Hugo</a> 强力驱动</p>
  <p class="footer-copyright">© 2026 Omar Alani. 保留所有权利。</p>
</div>

</div><!-- End .landing-page -->
