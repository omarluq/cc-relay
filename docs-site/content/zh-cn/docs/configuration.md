---
title: 配置
weight: 3
---

CC-Relay 通过 YAML 文件进行配置。本指南涵盖所有配置选项。

## 配置文件位置

默认位置（按顺序检查）：

1. `./config.yaml`（当前目录）
2. `~/.config/cc-relay/config.yaml`
3. 通过 `--config` 标志指定的路径

使用以下命令生成默认配置：

```bash
cc-relay config init
```

## 环境变量扩展

CC-Relay 支持使用 `${VAR_NAME}` 语法扩展环境变量：

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # 加载时扩展
```

## 完整配置参考

```yaml
# ==========================================================================
# 服务器配置
# ==========================================================================
server:
  # 监听地址
  listen: "127.0.0.1:8787"

  # 请求超时（毫秒），默认：600000 = 10 分钟
  timeout_ms: 600000

  # 最大并发请求数（0 = 无限制）
  max_concurrent: 0

  # 启用 HTTP/2 以提高性能
  enable_http2: true

  # 认证配置
  auth:
    # 代理访问所需的特定 API 密钥
    api_key: "${PROXY_API_KEY}"

    # 允许 Claude Code 订阅 Bearer Token
    allow_subscription: true

    # 要验证的特定 Bearer Token（可选）
    bearer_secret: "${BEARER_SECRET}"

# ==========================================================================
# 供应商配置
# ==========================================================================
providers:
  # Anthropic 直接 API
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # 可选，使用默认值

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60       # 每分钟请求数
        tpm_limit: 100000   # 每分钟 Token 数

    # 可选：指定可用模型
    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"

  # Z.AI / 智谱 GLM
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"

    keys:
      - key: "${ZAI_API_KEY}"

    # 将 Claude 模型名称映射到 Z.AI 模型
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    # 可选：指定可用模型
    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"

# ==========================================================================
# 日志配置
# ==========================================================================
logging:
  # 日志级别：debug, info, warn, error
  level: "info"

  # 日志格式：json, text
  format: "text"

  # 启用彩色输出（仅限 text 格式）
  pretty: true

  # 细粒度调试选项
  debug_options:
    log_request_body: false
    log_response_headers: false
    log_tls_metrics: false
    max_body_log_size: 1000
```

## 服务器配置

### 监听地址

`listen` 字段指定代理监听传入请求的位置：

```yaml
server:
  listen: "127.0.0.1:8787"  # 仅本地（推荐）
  # listen: "0.0.0.0:8787"  # 所有接口（谨慎使用）
```

### 认证

CC-Relay 支持多种认证方式：

#### API 密钥认证

要求客户端提供特定的 API 密钥：

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```

客户端必须包含请求头：`x-api-key: <your-proxy-key>`

#### Claude Code 订阅透传

允许 Claude Code 订阅用户连接：

```yaml
server:
  auth:
    allow_subscription: true
```

这将接受来自 Claude Code 的 `Authorization: Bearer` Token。

#### 组合认证

同时允许 API 密钥和订阅认证：

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```

#### 无认证

禁用认证（不推荐用于生产环境）：

```yaml
server:
  auth: {}
  # 或直接省略 auth 部分
```

### HTTP/2 支持

启用 HTTP/2 以提高并发请求性能：

```yaml
server:
  enable_http2: true
```

## 供应商配置

### 供应商类型

CC-Relay 目前支持两种供应商类型：

| 类型 | 描述 | 默认基础 URL |
|------|-------------|------------------|
| `anthropic` | Anthropic 直接 API | `https://api.anthropic.com` |
| `zai` | Z.AI / 智谱 GLM | `https://api.z.ai/api/anthropic` |

### Anthropic 供应商

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # 可选

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60
        tpm_limit: 100000

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```

### Z.AI 供应商

Z.AI 以较低成本提供与 Anthropic 兼容的 API 和 GLM 模型：

```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"

    keys:
      - key: "${ZAI_API_KEY}"

    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"
```

### 多个 API 密钥

池化多个 API 密钥以提高吞吐量：

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true

    keys:
      - key: "${ANTHROPIC_API_KEY_1}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_2}"
        rpm_limit: 60
        tpm_limit: 100000
      - key: "${ANTHROPIC_API_KEY_3}"
        rpm_limit: 60
        tpm_limit: 100000
```

### 自定义基础 URL

覆盖默认 API 端点：

```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```

## 日志配置

### 日志级别

| 级别 | 描述 |
|-------|-------------|
| `debug` | 详细输出，用于开发 |
| `info` | 正常操作消息 |
| `warn` | 警告消息 |
| `error` | 仅错误消息 |

### 日志格式

```yaml
logging:
  format: "text"   # 人类可读（默认）
  # format: "json" # 机器可读，用于日志聚合
```

### 调试选项

细粒度控制调试日志：

```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # 记录请求体（已脱敏）
    log_response_headers: true  # 记录响应头
    log_tls_metrics: true       # 记录 TLS 连接信息
    max_body_log_size: 1000     # 记录请求体的最大字节数
```

## 配置示例

### 最小单供应商配置

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

### 多供应商配置

```yaml
server:
  listen: "127.0.0.1:8787"
  auth:
    allow_subscription: true

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"

logging:
  level: "info"
  format: "text"
```

### 带调试日志的开发配置

```yaml
server:
  listen: "127.0.0.1:8787"

providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"

logging:
  level: "debug"
  format: "text"
  pretty: true
  debug_options:
    log_request_body: true
    log_response_headers: true
    log_tls_metrics: true
```

## 验证配置

验证配置文件：

```bash
cc-relay config validate
```

## 热重载

配置更改需要重启服务器。热重载功能计划在未来版本中实现。

## 下一步

- [了解架构](/zh/docs/architecture/)
- [API 参考](/zh/docs/api/)
