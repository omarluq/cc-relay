---
title: 文档
weight: 1
---

欢迎阅读 CC-Relay 文档！本指南将帮助您完成 CC-Relay 的安装、配置，并将其用作 Claude Code 和其他 LLM 客户端的多供应商代理。

## 什么是 CC-Relay？

CC-Relay 是一个使用 Go 语言编写的高性能 HTTP 代理，位于 LLM 客户端（如 Claude Code）和 LLM 供应商之间。它提供：

- **多供应商支持**：Anthropic 和 Z.AI（更多供应商即将支持）
- **Anthropic API 兼容**：即插即用，替代直接 API 访问
- **SSE 流式传输**：完整支持流式响应
- **多种认证方式**：支持 API 密钥和 Bearer Token
- **Claude Code 集成**：内置配置命令，轻松设置

## 当前状态

CC-Relay 正在积极开发中。当前已实现的功能：

| 功能 | 状态 |
|---------|--------|
| HTTP 代理服务器 | 已实现 |
| Anthropic 供应商 | 已实现 |
| Z.AI 供应商 | 已实现 |
| SSE 流式传输 | 已实现 |
| API 密钥认证 | 已实现 |
| Bearer Token（订阅）认证 | 已实现 |
| Claude Code 配置 | 已实现 |
| 多 API 密钥 | 已实现 |
| 调试日志 | 已实现 |

**计划功能：**
- 路由策略（轮询、故障转移、基于成本）
- 每个 API 密钥的速率限制
- 熔断器和健康追踪
- gRPC 管理 API
- TUI 仪表盘
- 更多供应商（Ollama、Bedrock、Azure、Vertex）

## 快速开始

```bash
# 安装
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest

# 初始化配置
cc-relay config init

# 设置 API 密钥
export ANTHROPIC_API_KEY="your-key-here"

# 启动代理
cc-relay serve

# 配置 Claude Code（在另一个终端）
cc-relay config cc init
```

## 快速导航

- [快速开始](/zh/docs/getting-started/) - 安装和首次运行
- [配置](/zh/docs/configuration/) - 供应商设置和选项
- [架构](/zh/docs/architecture/) - 系统设计和组件
- [API 参考](/zh/docs/api/) - HTTP 端点和示例

## 文档章节

### 快速开始
- [安装](/zh/docs/getting-started/#安装)
- [快速开始](/zh/docs/getting-started/#快速开始)
- [CLI 命令](/zh/docs/getting-started/#cli-命令)
- [使用 Claude Code 测试](/zh/docs/getting-started/#使用-claude-code-测试)
- [故障排除](/zh/docs/getting-started/#故障排除)

### 配置
- [服务器配置](/zh/docs/configuration/#服务器配置)
- [供应商配置](/zh/docs/configuration/#供应商配置)
- [认证](/zh/docs/configuration/#认证)
- [日志配置](/zh/docs/configuration/#日志配置)
- [配置示例](/zh/docs/configuration/#配置示例)

### 架构
- [系统概览](/zh/docs/architecture/#系统概览)
- [核心组件](/zh/docs/architecture/#核心组件)
- [请求流程](/zh/docs/architecture/#请求流程)
- [SSE 流式传输](/zh/docs/architecture/#sse-流式传输)
- [认证流程](/zh/docs/architecture/#认证流程)

### API 参考
- [POST /v1/messages](/zh/docs/api/#post-v1messages)
- [GET /v1/models](/zh/docs/api/#get-v1models)
- [GET /v1/providers](/zh/docs/api/#get-v1providers)
- [GET /health](/zh/docs/api/#get-health)
- [客户端示例](/zh/docs/api/#curl-示例)

## 需要帮助？

- [报告问题](https://github.com/omarluq/cc-relay/issues)
- [讨论](https://github.com/omarluq/cc-relay/discussions)
