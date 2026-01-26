---
title: 配置
weight: 3
---

CC-Relay 通过 YAML 或 TOML 文件进行配置。本指南涵盖所有配置选项。

## 配置文件位置

默认位置（按顺序检查）：

1. `./config.yaml` 或 `./config.toml`（当前目录）
2. `~/.config/cc-relay/config.yaml` 或 `~/.config/cc-relay/config.toml`
3. 通过 `--config` 标志指定的路径

文件格式根据扩展名（`.yaml`、`.yml` 或 `.toml`）自动检测。

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

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
# ==========================================================================
# Server Configuration
# ==========================================================================
[server]
# Address to listen on
listen = "127.0.0.1:8787"

# Request timeout in milliseconds (default: 600000 = 10 minutes)
timeout_ms = 600000

# Maximum concurrent requests (0 = unlimited)
max_concurrent = 0

# Enable HTTP/2 for better performance
enable_http2 = true

# Authentication configuration
[server.auth]
# Require specific API key for proxy access
api_key = "${PROXY_API_KEY}"

# Allow Claude Code subscription Bearer tokens
allow_subscription = true

# Specific Bearer token to validate (optional)
bearer_secret = "${BEARER_SECRET}"

# ==========================================================================
# Provider Configurations
# ==========================================================================

# Anthropic Direct API
[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true
base_url = "https://api.anthropic.com"  # Optional, uses default

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"
rpm_limit = 60       # Requests per minute
tpm_limit = 100000   # Tokens per minute

# Optional: Specify available models
models = [
  "claude-sonnet-4-5-20250514",
  "claude-opus-4-5-20250514",
  "claude-haiku-3-5-20241022"
]

# Z.AI / Zhipu GLM
[[providers]]
name = "zai"
type = "zai"
enabled = true
base_url = "https://api.z.ai/api/anthropic"

[[providers.keys]]
key = "${ZAI_API_KEY}"

# Map Claude model names to Z.AI models
[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"
"claude-haiku-3-5-20241022" = "GLM-4.5-Air"

# Optional: Specify available models
models = [
  "GLM-4.7",
  "GLM-4.5-Air",
  "GLM-4-Plus"
]

# ==========================================================================
# Logging Configuration
# ==========================================================================
[logging]
# Log level: debug, info, warn, error
level = "info"

# Log format: json, text
format = "text"

# Enable colored output (for text format)
pretty = true

# Granular debug options
[logging.debug_options]
log_request_body = false
log_response_headers = false
log_tls_metrics = false
max_body_log_size = 1000

# ==========================================================================
# Cache Configuration
# ==========================================================================
[cache]
# Cache mode: single, ha, disabled
mode = "single"

# Single mode (Ristretto) configuration
[cache.ristretto]
num_counters = 1000000  # 10x expected max items
max_cost = 104857600    # 100 MB
buffer_items = 64       # Admission buffer size

# HA mode (Olric) configuration
[cache.olric]
embedded = true                 # Run embedded Olric node
bind_addr = "0.0.0.0:3320"      # Olric client port
dmap_name = "cc-relay"          # Distributed map name
environment = "lan"             # local, lan, or wan
peers = ["other-node:3322"]     # Memberlist addresses (bind_addr + 2)
replica_count = 2               # Copies per key
read_quorum = 1                 # Min reads for success
write_quorum = 1                # Min writes for success
member_count_quorum = 2         # Min cluster members
leave_timeout = "5s"            # Leave broadcast duration

# ==========================================================================
# Routing Configuration
# ==========================================================================
[routing]
# Strategy: round_robin, weighted_round_robin, shuffle, failover (default)
strategy = "failover"

# Timeout for failover attempts in milliseconds (default: 5000)
failover_timeout = 5000

# Enable debug headers (X-CC-Relay-Strategy, X-CC-Relay-Provider)
debug = false
```
  {{< /tab >}}
{{< /tabs >}}

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

## 路由配置

CC-Relay 支持多种路由策略来分配跨供应商的请求。

```yaml
# ==========================================================================
# 路由配置
# ==========================================================================
routing:
  # 策略: round_robin, weighted_round_robin, shuffle, failover（默认）
  strategy: failover

  # 故障转移尝试的超时时间（毫秒，默认: 5000）
  failover_timeout: 5000

  # 启用调试头（X-CC-Relay-Strategy, X-CC-Relay-Provider）
  debug: false
```

### 路由策略

| 策略 | 描述 |
|------|------|
| `failover` | 按优先级顺序尝试供应商，失败时回退（默认） |
| `round_robin` | 顺序轮换供应商 |
| `weighted_round_robin` | 按权重比例分配 |
| `shuffle` | 公平随机分配 |

### 供应商权重和优先级

权重和优先级在供应商的第一个密钥中配置：

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # 用于 weighted-round-robin（数值越高 = 更多流量）
        priority: 2    # 用于 failover（数值越高 = 优先尝试）
```

有关策略说明、调试头和故障转移触发器的详细路由配置，请参阅[路由](/zh-cn/docs/routing/)。

## 配置示例

### 最小单供应商配置

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

### 多供应商配置

{{< tabs items="YAML,TOML" >}}
  {{< tab >}}
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
  {{< /tab >}}
  {{< tab >}}
```toml
[server]
listen = "127.0.0.1:8787"

[server.auth]
allow_subscription = true

[[providers]]
name = "anthropic"
type = "anthropic"
enabled = true

[[providers.keys]]
key = "${ANTHROPIC_API_KEY}"

[[providers]]
name = "zai"
type = "zai"
enabled = true

[[providers.keys]]
key = "${ZAI_API_KEY}"

[providers.model_mapping]
"claude-sonnet-4-5-20250514" = "GLM-4.7"

[logging]
level = "info"
format = "text"
```
  {{< /tab >}}
{{< /tabs >}}

### 带调试日志的开发配置

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

logging:
  level: "debug"
  format: "text"
  pretty: true
  debug_options:
    log_request_body: true
    log_response_headers: true
    log_tls_metrics: true
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

[logging]
level = "debug"
format = "text"
pretty = true

[logging.debug_options]
log_request_body = true
log_response_headers = true
log_tls_metrics = true
```
  {{< /tab >}}
{{< /tabs >}}

## 验证配置

验证配置文件：

```bash
cc-relay config validate
```

**提示**: 部署前始终验证配置更改。热重载会拒绝无效配置，但验证可以在到达生产环境之前捕获错误。

## 热重载

CC-Relay 自动检测并应用配置更改，无需重启。这使得可以在零停机时间内更新配置。

### 工作原理

CC-Relay 使用 [fsnotify](https://github.com/fsnotify/fsnotify) 监控配置文件：

1. **文件监控**：监控父目录以正确检测原子写入（大多数编辑器使用的临时文件+重命名模式）
2. **防抖动**：多个快速文件事件会以100毫秒延迟合并以处理编辑器保存行为
3. **原子交换**：新配置使用 Go 的 `sync/atomic.Pointer` 原子加载和交换
4. **保留进行中的请求**：正在处理的请求继续使用旧配置；新请求使用更新的配置

### 触发重载的事件

| 事件 | 触发重载 |
|------|---------|
| 文件写入 | 是 |
| 文件创建（原子重命名） | 是 |
| 文件 chmod | 否（忽略） |
| 目录中的其他文件 | 否（忽略） |

### 日志记录

热重载发生时，您将看到日志消息：

```
INF config file reloaded path=/path/to/config.yaml
INF config hot-reloaded successfully
```

如果新配置无效：

```
ERR failed to reload config path=/path/to/config.yaml error="validation error"
```

无效的配置会被拒绝，代理将继续使用之前的有效配置运行。

### 限制

- **提供者更改**：添加或删除提供者需要重启（路由基础设施在启动时初始化）
- **监听地址**：更改 `server.listen` 需要重启
- **gRPC 地址**：更改 gRPC 管理 API 地址需要重启

可以热重载的配置选项：
- 日志级别和格式
- 现有密钥的速率限制
- 健康检查间隔
- 路由策略权重和优先级

## 下一步

- [路由策略](/zh-cn/docs/routing/) - 供应商选择和故障转移
- [了解架构](/zh-cn/docs/architecture/)
- [API 参考](/zh-cn/docs/api/)
