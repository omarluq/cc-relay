---
title: 关于
type: about
---

## 关于 CC-Relay

CC-Relay 是一个使用 Go 语言编写的高性能 HTTP 代理，使 Claude Code 和其他 LLM 客户端能够通过单一端点连接多个供应商。

### 项目目标

- **简化多供应商访问** - 一个代理，多个后端
- **保持 API 兼容性** - 即插即用，替代直接访问 Anthropic API
- **实现灵活性** - 轻松切换供应商，无需修改客户端
- **支持 Claude Code** - 与 Claude Code CLI 的一流集成

### 当前状态

CC-Relay 正在积极开发中。以下功能已实现：

- 兼容 Anthropic API 的 HTTP 代理服务器
- Anthropic 和 Z.AI 供应商支持
- 完整的 SSE 流式传输支持
- API 密钥和 Bearer Token 认证
- 每个供应商支持多个 API 密钥
- 用于请求/响应检查的调试日志
- Claude Code 配置命令

### 计划功能

- 更多供应商支持（Ollama、AWS Bedrock、Azure、Vertex AI）
- 路由策略（轮询、故障转移、基于成本）
- 每个 API 密钥的速率限制
- 熔断器和健康追踪
- gRPC 管理 API
- TUI 仪表盘

### 技术栈

- [Go](https://go.dev/) - 编程语言
- [Cobra](https://cobra.dev/) - CLI 框架
- [zerolog](https://github.com/rs/zerolog) - 结构化日志

### 作者

由 [Omar Alani](https://github.com/omarluq) 创建

### 许可证

CC-Relay 是根据 [AGPL 3 许可证](https://github.com/omarluq/cc-relay/blob/main/LICENSE) 发布的开源软件。

### 贡献

欢迎贡献！请访问 [GitHub 仓库](https://github.com/omarluq/cc-relay) 了解更多：

- [报告问题](https://github.com/omarluq/cc-relay/issues)
- [提交 Pull Request](https://github.com/omarluq/cc-relay/pulls)
- [讨论](https://github.com/omarluq/cc-relay/discussions)
