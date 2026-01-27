---
title: 快速开始
weight: 2
---

本指南将引导您完成 CC-Relay 的安装、配置和首次运行。

## 前提条件

- **Go 1.21+** 用于从源码构建
- **API 密钥** 至少需要一个支持的供应商（Anthropic 或 Z.AI）
- **Claude Code** CLI 用于测试（可选）

## 安装

### 使用 Go Install

```bash
go install github.com/omarluq/cc-relay@latest
```

二进制文件将安装到 `$GOPATH/bin/cc-relay` 或 `$HOME/go/bin/cc-relay`。

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/omarluq/cc-relay.git
cd cc-relay

# 使用 task 构建（推荐）
task build

# 或手动构建
go build -o cc-relay ./cmd/cc-relay

# 运行
./cc-relay --help
```

### 预编译二进制文件

从[发布页面](https://github.com/omarluq/cc-relay/releases)下载预编译的二进制文件。

## 快速开始

### 1. 初始化配置

CC-Relay 可以为您生成默认配置文件：

```bash
cc-relay config init
```

这将在 `~/.config/cc-relay/config.yaml` 创建一个带有合理默认值的配置文件。

### 2. 设置环境变量

```bash
export ANTHROPIC_API_KEY="your-api-key-here"

# 可选：如果使用 Z.AI
export ZAI_API_KEY="your-zai-key-here"
```

### 3. 运行 CC-Relay

```bash
cc-relay serve
```

您应该看到类似以下的输出：

```
INF starting cc-relay listen=127.0.0.1:8787
INF using primary provider provider=anthropic-pool type=anthropic
```

### 4. 配置 Claude Code

配置 Claude Code 使用 CC-Relay 的最简单方法：

```bash
cc-relay config cc init
```

这将自动更新 `~/.claude/settings.json` 中的代理配置。

或者，手动设置环境变量：

```bash
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_AUTH_TOKEN="managed-by-cc-relay"
claude
```

## 验证是否正常工作

### 检查服务器状态

```bash
cc-relay status
```

输出：
```
✓ cc-relay is running (127.0.0.1:8787)
```

### 测试健康检查端点

```bash
curl http://localhost:8787/health
```

响应：
```json
{"status":"ok"}
```

### 列出可用模型

```bash
curl http://localhost:8787/v1/models
```

### 测试请求

```bash
curl -X POST http://localhost:8787/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-5-20250514",
    "max_tokens": 100,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

## CLI 命令

CC-Relay 提供以下 CLI 命令：

| 命令 | 描述 |
|---------|-------------|
| `cc-relay serve` | 启动代理服务器 |
| `cc-relay status` | 检查服务器是否运行 |
| `cc-relay config init` | 生成默认配置文件 |
| `cc-relay config cc init` | 配置 Claude Code 使用 cc-relay |
| `cc-relay config cc remove` | 从 Claude Code 移除 cc-relay 配置 |
| `cc-relay --version` | 显示版本信息 |

### Serve 命令选项

```bash
cc-relay serve [flags]

Flags:
  --config string      配置文件路径（默认：~/.config/cc-relay/config.yaml）
  --log-level string   日志级别（debug, info, warn, error）
  --log-format string  日志格式（json, text）
  --debug              启用调试模式（详细日志）
```

## 最小配置

以下是最小的可用配置：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8787"

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"

[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
```
  {{< /tab >}}
{{< /tabs >}}

## 下一步

- [配置多个供应商](/zh/docs/configuration/)
- [了解架构](/zh/docs/architecture/)
- [API 参考](/zh/docs/api/)

## 故障排除

### 端口已被占用

如果端口 8787 已被占用，在配置中更改监听地址：

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
```yaml
server:
  listen: "127.0.0.1:8788"
```
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8788"
```
  {{< /tab >}}
{{< /tabs >}}

### 供应商无响应

检查服务器日志中的连接错误：

```bash
cc-relay serve --log-level debug
```

### 认证错误

如果看到"authentication failed"错误：

1. 确认环境变量中的 API 密钥设置正确
2. 检查配置文件是否引用了正确的环境变量
3. 确保 API 密钥在供应商处有效

### 调试模式

启用调试模式以获取详细的请求/响应日志：

```bash
cc-relay serve --debug
```

这将启用：
- 调试日志级别
- 请求体日志（敏感字段已脱敏）
- 响应头日志
- TLS 连接指标
