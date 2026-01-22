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

# ==========================================================================
# 缓存配置
# ==========================================================================
cache:
  # 缓存模式: single, ha, disabled
  mode: single

  # 单机模式 (Ristretto) 配置
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size

  # HA模式 (Olric) 配置
  olric:
    embedded: true                 # Run embedded Olric node
    bind_addr: "0.0.0.0:3320"      # Olric client port
    dmap_name: "cc-relay"          # Distributed map name
    environment: lan               # local, lan, or wan
    peers:                         # Memberlist addresses (bind_addr + 2)
      - "other-node:3322"
    replica_count: 2               # Copies per key
    read_quorum: 1                 # Min reads for success
    write_quorum: 1                # Min writes for success
    member_count_quorum: 2         # Min cluster members
    leave_timeout: 5s              # Leave broadcast duration
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

## 缓存配置

CC-Relay 提供统一的缓存层，支持多种后端选项以适应不同的部署场景。

### 缓存模式

| 模式 | 后端 | 使用场景 |
|------|---------|----------|
| `single` | [Ristretto](https://github.com/dgraph-io/ristretto) | 单实例部署，高性能 |
| `ha` | [Olric](https://github.com/buraksezer/olric) | 多实例部署，共享状态 |
| `disabled` | Noop | 无缓存，直通 |

### 单机模式 (Ristretto)

Ristretto 是一个高性能、支持并发的内存缓存。这是单实例部署的默认模式。

```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size
```

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `num_counters` | int64 | 1,000,000 | 4位访问计数器的数量。推荐：预期最大条目数的10倍。 |
| `max_cost` | int64 | 104,857,600 (100 MB) | 缓存可容纳的最大内存（字节）。 |
| `buffer_items` | int64 | 64 | 每个 Get 缓冲区的键数。控制准入缓冲区大小。 |

### HA模式 (Olric) - 嵌入式

对于需要共享缓存状态的多实例部署，使用嵌入式 Olric 模式，每个 cc-relay 实例运行一个 Olric 节点。

```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "other-node:3322"  # Memberlist port = bind_addr + 2
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```

| 字段 | 类型 | 默认值 | 描述 |
|------|------|--------|------|
| `embedded` | bool | false | 运行嵌入式 Olric 节点 (true) vs. 连接到外部集群 (false)。 |
| `bind_addr` | string | 必需 | Olric 客户端连接地址（例如 "0.0.0.0:3320"）。 |
| `dmap_name` | string | "cc-relay" | 分布式映射的名称。所有节点必须使用相同的名称。 |
| `environment` | string | "local" | Memberlist 预设："local"、"lan" 或 "wan"。 |
| `peers` | []string | - | 用于对等发现的 Memberlist 地址。使用端口 bind_addr + 2。 |
| `replica_count` | int | 1 | 每个键的副本数。1 = 无复制。 |
| `read_quorum` | int | 1 | 响应所需的最小成功读取数。 |
| `write_quorum` | int | 1 | 响应所需的最小成功写入数。 |
| `member_count_quorum` | int32 | 1 | 运行所需的最小集群成员数。 |
| `leave_timeout` | duration | 5s | 关闭前广播离开消息的时间。 |

**重要：** Olric 使用两个端口 - 用于客户端连接的 `bind_addr` 端口和用于 memberlist 通信的 `bind_addr + 2`。请确保防火墙开放这两个端口。

### HA模式 (Olric) - 客户端模式

连接到外部 Olric 集群，而不是运行嵌入式节点：

```yaml
cache:
  mode: ha
  olric:
    embedded: false
    addresses:
      - "olric-node-1:3320"
      - "olric-node-2:3320"
    dmap_name: "cc-relay"
```

| 字段 | 类型 | 描述 |
|------|------|------|
| `embedded` | bool | 客户端模式设置为 `false`。 |
| `addresses` | []string | 外部 Olric 集群地址。 |
| `dmap_name` | string | 分布式映射名称（必须与集群配置匹配）。 |

### 禁用模式

完全禁用缓存，用于调试或在其他地方处理缓存：

```yaml
cache:
  mode: disabled
```

有关包括HA集群指南和故障排除在内的完整缓存文档，请参阅[缓存](/zh-cn/docs/caching/)。

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
