---
title: 路由
weight: 4
---

CC-Relay 支持多种路由策略来分配跨供应商的请求。本页介绍每种策略及其配置方法。

## 概述

路由决定了 cc-relay 如何选择哪个供应商处理每个请求。正确的策略取决于您的优先级：可用性、成本、延迟或负载分配。

| 策略 | 配置值 | 描述 | 用例 |
|------|--------|------|------|
| Round-Robin | `round_robin` | 顺序轮换供应商 | 均匀分配 |
| Weighted Round-Robin | `weighted_round_robin` | 按权重比例分配 | 基于容量的分配 |
| Shuffle | `shuffle` | 公平随机（"发牌"模式） | 随机化负载均衡 |
| Failover | `failover`（默认） | 基于优先级的自动重试 | 高可用性 |
| Model-Based | `model_based` | 按模型名称前缀路由 | 多模型部署 |

## 配置

在 `config.yaml` 中配置路由：

```yaml
routing:
  # 策略: round_robin, weighted_round_robin, shuffle, failover（默认）, model_based
  strategy: failover

  # 故障转移尝试的超时时间（毫秒，默认: 5000）
  failover_timeout: 5000

  # 启用调试头（X-CC-Relay-Strategy, X-CC-Relay-Provider）
  debug: false

  # 基于模型的路由配置（仅在 strategy: model_based 时使用）
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
  default_provider: anthropic
```

**默认值:** 如果未指定 `strategy`，cc-relay 将使用 `failover` 作为最安全的选项。

## 策略

### Round-Robin

使用原子计数器进行顺序分配。在任何供应商收到第二个请求之前，每个供应商都会收到一个请求。

```yaml
routing:
  strategy: round_robin
```

**工作原理:**

1. 请求 1 → 供应商 A
2. 请求 2 → 供应商 B
3. 请求 3 → 供应商 C
4. 请求 4 → 供应商 A（循环重复）

**最佳用途:** 在容量相近的供应商之间均匀分配。

### Weighted Round-Robin

根据供应商权重按比例分配请求。使用 Nginx smooth weighted round-robin 算法实现均匀分配。

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3  # 接收 3 倍的请求

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        weight: 1  # 接收 1 倍的请求
```

**工作原理:**

权重为 3:1 时，每 4 个请求：
- 3 个请求 → anthropic
- 1 个请求 → zai

**默认权重:** 1（如果未指定）

**最佳用途:** 根据供应商容量、速率限制或成本分配进行负载分配。

### Shuffle

使用 Fisher-Yates "发牌" 模式的公平随机分配。每个人都拿到一张牌后，才有人拿第二张。

```yaml
routing:
  strategy: shuffle
```

**工作原理:**

1. 所有供应商进入一个"牌堆"
2. 随机选择并移除一个供应商
3. 牌堆为空时，重新洗牌所有供应商
4. 保证随时间推移的公平分配

**最佳用途:** 在确保公平性的同时进行随机化负载均衡。

### Failover

按优先级顺序尝试供应商。失败时，并行请求剩余供应商以获得最快的成功响应。这是**默认策略**。

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # 首先尝试（数值越高 = 优先级越高）

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # 备用
```

**工作原理:**

1. 首先尝试最高优先级的供应商
2. 如果失败（参见[故障转移触发器](#故障转移触发器)），向所有剩余供应商发起并行请求
3. 返回第一个成功的响应，取消其他请求
4. 遵循 `failover_timeout` 的总操作时间

**默认优先级:** 1（如果未指定）

**最佳用途:** 带自动故障转移的高可用性。

### Model-Based

根据请求中的模型名称将请求路由到供应商。使用最长前缀匹配以提高特异性。

```yaml
routing:
  strategy: model_based
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
    llama: ollama
  default_provider: anthropic

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
  - name: "ollama"
    type: "ollama"
    base_url: "http://localhost:11434"
```

**工作原理:**

1. 从请求中提取 `model` 参数
2. 尝试在 `model_mapping` 中找到最长前缀匹配
3. 路由到相应的供应商
4. 如果未找到匹配则回退到 `default_provider`
5. 如果既没有匹配也没有默认值则返回错误

**前缀匹配示例:**

| 请求的模型 | 映射条目 | 选定条目 | 供应商 |
|-----------|----------|---------|--------|
| `claude-opus-4` | `claude-opus`, `claude` | `claude-opus` | anthropic |
| `claude-sonnet-3.5` | `claude-sonnet`, `claude` | `claude-sonnet` | anthropic |
| `glm-4-plus` | `glm-4`, `glm` | `glm-4` | zai |
| `qwen-72b` | `qwen`, `claude` | `qwen` | ollama |
| `llama-3.2` | `llama`, `claude` | `llama` | ollama |
| `gpt-4` | `claude`, `llama` | (无匹配) | default_provider |

**最佳用途:** 需要将不同模型路由到不同供应商的多模型部署。

## 调试头

当 `routing.debug: true` 时，cc-relay 会在响应中添加诊断头：

| 头部 | 值 | 描述 |
|------|-----|------|
| `X-CC-Relay-Strategy` | 策略名称 | 使用的路由策略 |
| `X-CC-Relay-Provider` | 供应商名称 | 处理请求的供应商 |

**响应头示例:**

```
X-CC-Relay-Strategy: failover
X-CC-Relay-Provider: anthropic
```

**安全警告:** 调试头会暴露内部路由决策。仅在开发或受信任的环境中使用。切勿在有不受信任客户端的生产环境中启用。

## 故障转移触发器

failover 策略在特定错误条件下触发重试：

| 触发器 | 条件 | 描述 |
|--------|------|------|
| 状态码 | `429`, `500`, `502`, `503`, `504` | 速率限制或服务器错误 |
| 超时 | `context.DeadlineExceeded` | 请求超时 |
| 连接 | `net.Error` | 网络错误、DNS 失败、连接被拒绝 |

**重要:** 客户端错误（除 429 外的 4xx）**不会**触发故障转移。这些错误表示请求本身有问题，而非供应商问题。

### 状态码说明

| 状态码 | 含义 | 触发故障转移? |
|--------|------|--------------|
| `429` | 速率受限 | 是 - 尝试其他供应商 |
| `500` | 内部服务器错误 | 是 - 服务器问题 |
| `502` | Bad Gateway | 是 - 上游问题 |
| `503` | 服务不可用 | 是 - 暂时宕机 |
| `504` | Gateway Timeout | 是 - 上游超时 |
| `400` | Bad Request | 否 - 修复请求 |
| `401` | Unauthorized | 否 - 修复认证 |
| `403` | Forbidden | 否 - 权限问题 |

## 示例

### 简单 Failover（推荐大多数用户使用）

使用带优先级供应商的默认策略：

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1
```

### 加权负载均衡

根据供应商容量分配负载：

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "primary"
    type: "anthropic"
    keys:
      - key: "${PRIMARY_KEY}"
        weight: 3  # 75% 的流量

  - name: "secondary"
    type: "anthropic"
    keys:
      - key: "${SECONDARY_KEY}"
        weight: 1  # 25% 的流量
```

### 带调试头的开发环境

启用调试头进行故障排除：

```yaml
routing:
  strategy: round_robin
  debug: true

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
```

### 快速故障转移的高可用性

最小化故障转移延迟：

```yaml
routing:
  strategy: failover
  failover_timeout: 3000  # 3 秒超时

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1
```

### 使用基于模型路由的多模型

将不同模型路由到专用供应商：

```yaml
routing:
  strategy: model_based
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
    llama: ollama
  default_provider: anthropic

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"

  - name: "ollama"
    type: "ollama"
    base_url: "http://localhost:11434"
```

通过此配置：
- Claude 模型 → Anthropic
- GLM 模型 → Z.AI
- Qwen/Llama 模型 → Ollama（本地）
- 其他模型 → Anthropic（默认）

## 供应商权重和优先级

权重和优先级在供应商的密钥配置中指定：

```yaml
providers:
  - name: "example"
    type: "anthropic"
    keys:
      - key: "${API_KEY}"
        weight: 3      # 用于 weighted-round-robin（数值越高 = 更多流量）
        priority: 2    # 用于 failover（数值越高 = 优先尝试）
        rpm_limit: 60  # 速率限制跟踪
```

**注意:** 权重和优先级从供应商密钥列表的**第一个密钥**读取。

## 后续步骤

- [配置参考](/zh-cn/docs/configuration/) - 完整配置选项
- [架构概述](/zh-cn/docs/architecture/) - cc-relay 内部工作原理
