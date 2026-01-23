---
title: "Providers"
description: "Configure Anthropic, Z.AI, and Ollama providers in cc-relay"
weight: 5
---

CC-Relay supports multiple LLM providers through a unified interface. This page explains how to configure each provider.

## Overview

CC-Relay acts as a proxy between Claude Code and various LLM backends. All providers expose an Anthropic-compatible Messages API, enabling seamless switching between providers.

| Provider | Type | Description | Cost |
|----------|------|-------------|------|
| Anthropic | `anthropic` | Direct Anthropic API access | Standard Anthropic pricing |
| Z.AI | `zai` | Zhipu AI GLM models, Anthropic-compatible | ~1/7 of Anthropic pricing |
| Ollama | `ollama` | Local LLM inference | Free (local compute) |

**Coming in Phase 6:** AWS Bedrock, Azure Foundry, Google Vertex AI

## Anthropic Provider

The Anthropic provider connects directly to Anthropic's API. This is the default provider for full Claude model access.

### Configuration

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # Optional, uses default

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60        # Requests per minute
        tpm_limit: 100000    # Tokens per minute
        priority: 2          # Higher = tried first in failover

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```

### API Key Setup

1. Create an account at [console.anthropic.com](https://console.anthropic.com)
2. Navigate to Settings > API Keys
3. Create a new API key
4. Store in environment variable: `export ANTHROPIC_API_KEY="sk-ant-..."`

### Transparent Auth Support

The Anthropic provider supports transparent authentication for Claude Code subscription users. When enabled, cc-relay forwards your subscription token unchanged:

```yaml
server:
  auth:
    allow_subscription: true
```

```bash
# Your subscription token flows through unchanged
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

See [Transparent Authentication](/docs/configuration/#transparent-authentication) for details.

## Z.AI Provider

Z.AI (Zhipu AI) offers GLM models through an Anthropic-compatible API. This provides significant cost savings (~1/7 of Anthropic pricing) while maintaining API compatibility.

### Configuration

```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"  # Optional, uses default

    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Lower priority than Anthropic for failover

    # Map Claude model names to Z.AI models
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"
      "claude-haiku-3-5": "GLM-4.5-Air"

    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"
```

### API Key Setup

1. Create an account at [open.bigmodel.cn](https://open.bigmodel.cn) (Zhipu AI Developer Portal)
2. Navigate to API Keys section
3. Create a new API key
4. Store in environment variable: `export ZAI_API_KEY="..."`

### Model Mapping

Model mapping translates Anthropic model names to Z.AI equivalents. When Claude Code requests `claude-sonnet-4-5-20250514`, cc-relay automatically routes to `GLM-4.7`:

```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7 (flagship model)
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air (fast, economical)
  "claude-haiku-3-5-20241022": "GLM-4.5-Air"
  "claude-haiku-3-5": "GLM-4.5-Air"
```

### Cost Comparison

| Model | Anthropic (per 1M tokens) | Z.AI Equivalent | Z.AI Cost |
|-------|---------------------------|-----------------|-----------|
| claude-sonnet-4-5 | $3 input / $15 output | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 input / $1.25 output | GLM-4.5-Air | ~$0.04 / $0.18 |

*Prices are approximate and subject to change.*

## Ollama Provider

Ollama enables local LLM inference through an Anthropic-compatible API (available since Ollama v0.14). Run models locally for privacy, zero API costs, and offline operation.

### Configuration

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"  # Optional, uses default

    keys:
      - key: "ollama"  # Ollama accepts but ignores API keys
        priority: 0    # Lowest priority for failover

    # Map Claude model names to local Ollama models
    model_mapping:
      "claude-sonnet-4-5-20250514": "qwen3:32b"
      "claude-sonnet-4-5": "qwen3:32b"
      "claude-haiku-3-5-20241022": "qwen3:8b"
      "claude-haiku-3-5": "qwen3:8b"

    models:
      - "qwen3:32b"
      - "qwen3:8b"
      - "codestral:latest"
```

### Ollama Setup

1. Install Ollama from [ollama.com](https://ollama.com)
2. Pull models you want to use:
   ```bash
   ollama pull qwen3:32b
   ollama pull qwen3:8b
   ollama pull codestral:latest
   ```
3. Start Ollama (runs automatically on install)

### Recommended Models

For Claude Code workflows, choose models with at least 32K context:

| Model | Context | Size | Best For |
|-------|---------|------|----------|
| `qwen3:32b` | 128K | 32B params | General coding, complex reasoning |
| `qwen3:8b` | 128K | 8B params | Fast iteration, simpler tasks |
| `codestral:latest` | 32K | 22B params | Code generation, specialized coding |
| `llama3.2:3b` | 128K | 3B params | Very fast, basic tasks |

### Feature Limitations

Ollama's Anthropic compatibility is partial. Some features are not supported:

| Feature | Supported | Notes |
|---------|-----------|-------|
| Streaming (SSE) | Yes | Same event sequence as Anthropic |
| Tool calling | Yes | Same format as Anthropic |
| Extended thinking | Partial | `budget_tokens` accepted but not enforced |
| Prompt caching | No | `cache_control` blocks ignored |
| PDF input | No | Not supported |
| Image URLs | No | Base64 encoding only |
| Token counting | No | `/v1/messages/count_tokens` not available |
| `tool_choice` | No | Cannot force specific tool usage |

### Docker Networking

When running cc-relay in Docker but Ollama on the host:

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # Use Docker's host gateway instead of localhost
    base_url: "http://host.docker.internal:11434"
```

Alternatively, run cc-relay with `--network host`:

```bash
docker run --network host cc-relay
```

## Model Mapping

The `model_mapping` field translates incoming model names to provider-specific models:

```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # Format: "incoming-model": "provider-model"
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
```

When Claude Code sends:
```json
{"model": "claude-sonnet-4-5-20250514", ...}
```

CC-Relay routes to Z.AI with:
```json
{"model": "GLM-4.7", ...}
```

### Mapping Tips

1. **Include version suffixes**: Map both `claude-sonnet-4-5` and `claude-sonnet-4-5-20250514`
2. **Consider context length**: Match models with similar capabilities
3. **Test quality**: Verify output quality matches your needs

## Multi-Provider Setup

Configure multiple providers for failover, cost optimization, or load distribution:

```yaml
providers:
  # Primary: Anthropic (highest quality)
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # Tried first

  # Secondary: Z.AI (cost-effective)
  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # Fallback

  # Tertiary: Ollama (local, free)
  - name: "ollama"
    type: "ollama"
    enabled: true
    keys:
      - key: "ollama"
        priority: 0  # Last resort

routing:
  strategy: failover  # Try providers in priority order
```

With this configuration:
1. Requests go to Anthropic first (priority 2)
2. If Anthropic fails (429, 5xx), try Z.AI (priority 1)
3. If Z.AI fails, try Ollama (priority 0)

See [Routing Strategies](/docs/routing/) for more options.

## Troubleshooting

### Connection Refused (Ollama)

**Symptom:** `connection refused` when connecting to Ollama

**Causes:**
- Ollama not running
- Wrong port
- Docker networking issue

**Solutions:**
```bash
# Check if Ollama is running
ollama list

# Verify port
curl http://localhost:11434/api/version

# For Docker, use host gateway
base_url: "http://host.docker.internal:11434"
```

### Authentication Failed (Z.AI)

**Symptom:** `401 Unauthorized` from Z.AI

**Causes:**
- Invalid API key
- Environment variable not set
- Key not activated

**Solutions:**
```bash
# Verify environment variable is set
echo $ZAI_API_KEY

# Test key directly
curl -X POST https://api.z.ai/api/anthropic/v1/messages \
  -H "x-api-key: $ZAI_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"GLM-4.7","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### Model Not Found

**Symptom:** `model not found` errors

**Causes:**
- Model not configured in `models` list
- Missing `model_mapping` entry
- Model not installed (Ollama)

**Solutions:**
```yaml
# Ensure model is listed
models:
  - "GLM-4.7"

# Ensure mapping exists
model_mapping:
  "claude-sonnet-4-5": "GLM-4.7"
```

For Ollama, verify model is installed:
```bash
ollama list
ollama pull qwen3:32b
```

### Slow Response (Ollama)

**Symptom:** Very slow responses from Ollama

**Causes:**
- Model too large for hardware
- GPU not being used
- Insufficient RAM

**Solutions:**
- Use smaller model (`qwen3:8b` instead of `qwen3:32b`)
- Verify GPU is enabled: `ollama run qwen3:8b --verbose`
- Check memory usage during inference

## Next Steps

- [Configuration Reference](/docs/configuration/) - Complete configuration options
- [Routing Strategies](/docs/routing/) - Provider selection and failover
- [Health Monitoring](/docs/health/) - Circuit breakers and health checks
