---
title: CC-Relay
layout: hextra-home
---

<div class="landing-page">

<div class="custom-hero">
  <h1 class="hero-title">CC-Relay</h1>
  <p class="hero-subtitle">
    Claude Code のマルチモデル体験を再定義する
  </p>
  <div class="hero-buttons">
    <a href="docs/getting-started/" class="hero-button hero-button-primary">はじめる</a>
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
    title="マルチプロバイダー対応"
    subtitle="単一のエンドポイントから Anthropic と Z.AI に接続（他のプロバイダーも近日対応予定）"
  >}}
  {{< hextra/feature-card
    title="SSE ストリーミング"
    subtitle="Claude Code と完全互換の Server-Sent Events をフルサポート"
  >}}
  {{< hextra/feature-card
    title="複数 API キー"
    subtitle="プロバイダーごとに複数の API キーをプールして高スループットを実現"
  >}}
  {{< hextra/feature-card
    title="柔軟な認証"
    subtitle="Claude Code サブスクリプションユーザー向けの API キーと Bearer トークンをサポート"
  >}}
  {{< hextra/feature-card
    title="Claude Code 統合"
    subtitle="組み込みの設定管理でワンコマンドセットアップ"
  >}}
  {{< hextra/feature-card
    title="Anthropic API 互換"
    subtitle="クライアント側の変更不要でそのまま置き換え可能"
  >}}
{{< /hextra/feature-grid >}}
</div>

<div class="info-box">
  <div class="info-box-title">
    <span class="info-icon">⚡</span>
    現在の機能
  </div>
  <div class="info-box-content">
    <div class="feature-item">
      <span class="feature-icon">🔑</span>
      <div>
        <strong>複数 API キー</strong>
        <p>プロバイダーごとに複数のキーをプールして高スループットを実現</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🔐</span>
      <div>
        <strong>マルチ認証サポート</strong>
        <p>柔軟なアクセスのための API キーと Bearer トークン認証</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">📡</span>
      <div>
        <strong>SSE ストリーミング</strong>
        <p>Claude Code のリアルタイムストリーミングと完全互換</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🎛️</span>
      <div>
        <strong>デバッグログ</strong>
        <p>トラブルシューティングのための詳細なリクエスト/レスポンスログ</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">⚙️</span>
      <div>
        <strong>環境変数</strong>
        <p>YAML で ${VAR} 展開を使用した安全な設定</p>
      </div>
    </div>
    <div class="feature-item">
      <span class="feature-icon">🚀</span>
      <div>
        <strong>簡単セットアップ</strong>
        <p>cc-relay config cc init でワンコマンド Claude Code 統合</p>
      </div>
    </div>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">クイックスタート</h2>

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
      <span class="terminal-command typing-1"># インストール</span>
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
      <span class="terminal-command typing-4"># 設定を初期化</span>
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
      <span class="terminal-command typing-7"># Claude Code 統合を設定</span>
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
      <span class="terminal-command typing-10"># サーバーを起動</span>
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
      <span class="terminal-command typing-13"># Claude Code を使い始める</span>
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
  <h2 class="section-title">アーキテクチャ</h2>
  <p class="section-description">CC-Relay は LLM クライアントとバックエンドプロバイダーの間に位置し、完全な Anthropic API 互換性でリクエストをプロキシします</p>

<div class="architecture-diagram">
  <div class="arch-layer">
    <div class="arch-layer-title">クライアント層</div>
    <div class="arch-node arch-node-client">
      <div class="client-icon">🤖</div>
      <div class="client-text">
        <strong>Claude Code クライアント</strong><br/>
        <span style="font-size: 0.875rem; opacity: 0.9;">POST /v1/messages</span>
      </div>
    </div>
  </div>

  <div class="arch-connector">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title">プロキシエンジン</div>
    <div class="arch-proxy">
      <div class="arch-proxy-component">🔐 認証</div>
      <div class="arch-proxy-component">📝 リクエストログ</div>
      <div class="arch-proxy-component">📡 SSE ストリーミング</div>
      <div class="arch-proxy-component">🔑 API キー管理</div>
    </div>
  </div>

  <div class="arch-connector">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title">プロバイダー層（実装済み）</div>
    <div class="arch-providers">
      <div class="arch-provider anthropic">
        <img src="/cc-relay/logos/anthropic.svg" alt="Anthropic" class="arch-provider-logo" />
        <div class="arch-provider-name">Anthropic</div>
        <div class="arch-provider-desc">Claude モデル</div>
      </div>
      <div class="arch-provider zai">
        <img src="/cc-relay/logos/zai.svg" alt="Z.AI" class="arch-provider-logo" />
        <div class="arch-provider-name">Z.AI</div>
        <div class="arch-provider-desc">GLM モデル</div>
      </div>
    </div>
  </div>

  <div class="arch-connector" style="margin-top: 1rem;">↓</div>

  <div class="arch-layer">
    <div class="arch-layer-title" style="opacity: 0.7;">近日対応予定</div>
    <div class="arch-providers" style="opacity: 0.6;">
      <div class="arch-provider ollama">
        <img src="/cc-relay/logos/ollama.svg" alt="Ollama" class="arch-provider-logo" />
        <div class="arch-provider-name">Ollama</div>
        <div class="arch-provider-desc">ローカルモデル</div>
      </div>
      <div class="arch-provider bedrock">
        <img src="/cc-relay/logos/aws.svg" alt="AWS Bedrock" class="arch-provider-logo" />
        <div class="arch-provider-name">AWS Bedrock</div>
        <div class="arch-provider-desc">SigV4 認証</div>
      </div>
      <div class="arch-provider azure">
        <img src="/cc-relay/logos/azure.svg" alt="Azure" class="arch-provider-logo" />
        <div class="arch-provider-name">Azure Foundry</div>
        <div class="arch-provider-desc">デプロイメント</div>
      </div>
      <div class="arch-provider vertex">
        <img src="/cc-relay/logos/gcp.svg" alt="Vertex AI" class="arch-provider-logo" />
        <div class="arch-provider-name">Vertex AI</div>
        <div class="arch-provider-desc">OAuth</div>
      </div>
    </div>
  </div>
</div>
</div>

<div class="section-box">
  <h2 class="section-title">ユースケース</h2>
  <p class="section-description">CC-Relay で開発ワークフローを強化</p>

  <div class="use-cases-grid">
    <div class="use-case-card">
      <div class="use-case-icon">🔄</div>
      <h3>プロバイダーの柔軟性</h3>
      <p>クライアントコードを変更せずに Anthropic と Z.AI を切り替え</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">👥</div>
      <h3>開発チーム</h3>
      <p>プールされたキーで複数の開発者間で API クォータを共有</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">🔑</div>
      <h3>API キー管理</h3>
      <p>クライアントの更新なしで API キーを一元管理およびローテーション</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">💰</div>
      <h3>コスト比較</h3>
      <p>Z.AI の GLM モデルを低コストの代替として試用</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">🔐</div>
      <h3>サブスクリプションパススルー</h3>
      <p>Claude Code サブスクリプションユーザーをプロキシ経由でルーティング</p>
    </div>
    <div class="use-case-card">
      <div class="use-case-icon">🐛</div>
      <h3>リクエストデバッグ</h3>
      <p>トラブルシューティングのために API リクエストをログ記録・検査</p>
    </div>
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">ドキュメント</h2>
  <p class="section-description">CC-Relay を始めてマスターするために必要なすべて</p>

  <div class="docs-grid">
    {{< hextra/feature-card
      title="はじめに"
      subtitle="インストール、設定、初回実行"
      link="/cc-relay/ja/docs/getting-started/"
    >}}
    {{< hextra/feature-card
      title="設定"
      subtitle="プロバイダーのセットアップ、ルーティング戦略、詳細オプション"
      link="/cc-relay/ja/docs/configuration/"
    >}}
    {{< hextra/feature-card
      title="アーキテクチャ"
      subtitle="システム設計、コンポーネント、API 互換性"
      link="/cc-relay/ja/docs/architecture/"
    >}}
    {{< hextra/feature-card
      title="API リファレンス"
      subtitle="HTTP エンドポイント、ストリーミング、クライアント例"
      link="/cc-relay/ja/docs/api/"
    >}}
  </div>
</div>

<div class="section-box">
  <h2 class="section-title">コントリビューション</h2>
  <p class="section-description">CC-Relay はオープンソースです！コントリビューションを歓迎します</p>

  <div class="contributing-links">
    <a href="https://github.com/omarluq/cc-relay/issues" class="contrib-link">
      <span class="contrib-icon">🐛</span>
      <span>バグを報告</span>
    </a>
    <a href="https://github.com/omarluq/cc-relay/issues" class="contrib-link">
      <span class="contrib-icon">💡</span>
      <span>機能をリクエスト</span>
    </a>
    <a href="https://github.com/omarluq/cc-relay/pulls" class="contrib-link">
      <span class="contrib-icon">🚀</span>
      <span>PR を送信</span>
    </a>
  </div>

  <div class="license-box">
    <p>AGPL 3 ライセンス - 詳細は <a href="https://github.com/omarluq/cc-relay/blob/main/LICENSE">LICENSE</a> をご覧ください</p>
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
  <p class="footer-powered">Powered by <a href="https://gohugo.io" target="_blank" rel="noopener">Hugo</a></p>
  <p class="footer-copyright">© 2026 Omar Alani. All rights reserved.</p>
</div>

</div><!-- End .landing-page -->
