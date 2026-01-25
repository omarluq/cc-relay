---
title: "프로바이더"
description: "cc-relay에서 Anthropic, Z.AI, Ollama 프로바이더 설정"
weight: 5
---

CC-Relay는 통합 인터페이스를 통해 여러 LLM 프로바이더를 지원합니다. 이 페이지에서는 각 프로바이더의 설정 방법을 설명합니다.

## 개요

CC-Relay는 Claude Code와 다양한 LLM 백엔드 사이의 프록시 역할을 합니다. 모든 프로바이더는 Anthropic 호환 Messages API를 제공하여 프로바이더 간 원활한 전환이 가능합니다.

| 프로바이더 | 타입 | 설명 | 비용 |
|-----------|------|------|------|
| Anthropic | `anthropic` | 직접 Anthropic API 접근 | 표준 Anthropic 가격 |
| Z.AI | `zai` | Zhipu AI GLM 모델, Anthropic 호환 | Anthropic 가격의 약 1/7 |
| Ollama | `ollama` | 로컬 LLM 추론 | 무료 (로컬 컴퓨팅) |
| AWS Bedrock | `bedrock` | SigV4 인증으로 AWS 경유 Claude | AWS Bedrock 가격 |
| Azure AI Foundry | `azure` | Azure MAAS 경유 Claude | Azure AI 가격 |
| Google Vertex AI | `vertex` | Google Cloud 경유 Claude | Vertex AI 가격 |

## Anthropic 프로바이더

Anthropic 프로바이더는 Anthropic의 API에 직접 연결합니다. Claude 모델에 대한 완전한 접근을 위한 기본 프로바이더입니다.

### 설정

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # 선택사항, 기본값 사용

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60        # 분당 요청 수
        tpm_limit: 100000    # 분당 토큰 수
        priority: 2          # 높음 = 장애 조치에서 먼저 시도

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```

### API 키 설정

1. [console.anthropic.com](https://console.anthropic.com)에서 계정 생성
2. Settings > API Keys로 이동
3. 새 API 키 생성
4. 환경 변수에 저장: `export ANTHROPIC_API_KEY="sk-ant-..."`

### 투명 인증 지원

Anthropic 프로바이더는 Claude Code 구독 사용자의 투명 인증을 지원합니다. 활성화하면 cc-relay가 구독 토큰을 변경 없이 전달합니다:

```yaml
server:
  auth:
    allow_subscription: true
```

```bash
# 구독 토큰이 변경 없이 전달됩니다
export ANTHROPIC_BASE_URL="http://localhost:8787"
claude
```

자세한 내용은 [투명 인증](/ko/docs/configuration/#투명-인증)을 참조하세요.

## Z.AI 프로바이더

Z.AI(Zhipu AI)는 Anthropic 호환 API를 통해 GLM 모델을 제공합니다. API 호환성을 유지하면서 상당한 비용 절감(Anthropic 가격의 약 1/7)을 제공합니다.

### 설정

```yaml
providers:
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"  # 선택사항, 기본값 사용

    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # 장애 조치 시 Anthropic보다 낮은 우선순위

    # Claude 모델 이름을 Z.AI 모델에 매핑
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

### API 키 설정

1. [z.ai/model-api](https://z.ai/model-api)에서 계정 생성
2. API Keys 섹션으로 이동
3. 새 API 키 생성
4. 환경 변수에 저장: `export ZAI_API_KEY="..."`

> **10% 할인:** 구독 시 [이 초대 링크](https://z.ai/subscribe?ic=HT5TQVSOZP)를 사용하면 본인과 추천인 모두 10% 할인을 받을 수 있습니다.

### Model Mapping

Model Mapping은 Anthropic 모델 이름을 Z.AI 동등 모델로 변환합니다. Claude Code가 `claude-sonnet-4-5-20250514`를 요청하면 cc-relay가 자동으로 `GLM-4.7`로 라우팅합니다:

```yaml
model_mapping:
  # Claude Sonnet -> GLM-4.7 (플래그십 모델)
  "claude-sonnet-4-5-20250514": "GLM-4.7"
  "claude-sonnet-4-5": "GLM-4.7"

  # Claude Haiku -> GLM-4.5-Air (빠름, 경제적)
  "claude-haiku-3-5-20241022": "GLM-4.5-Air"
  "claude-haiku-3-5": "GLM-4.5-Air"
```

### 비용 비교

| 모델 | Anthropic (백만 토큰당) | Z.AI 동등 | Z.AI 비용 |
|------|------------------------|----------|----------|
| claude-sonnet-4-5 | $3 입력 / $15 출력 | GLM-4.7 | ~$0.43 / $2.14 |
| claude-haiku-3-5 | $0.25 입력 / $1.25 출력 | GLM-4.5-Air | ~$0.04 / $0.18 |

*가격은 대략적이며 변경될 수 있습니다.*

## Ollama 프로바이더

Ollama는 Anthropic 호환 API(Ollama v0.14 이후 사용 가능)를 통해 로컬 LLM 추론을 가능하게 합니다. 프라이버시, API 비용 없음, 오프라인 운영을 위해 로컬에서 모델을 실행합니다.

### 설정

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    enabled: true
    base_url: "http://localhost:11434"  # 선택사항, 기본값 사용

    keys:
      - key: "ollama"  # Ollama는 API 키를 받지만 무시함
        priority: 0    # 장애 조치의 최저 우선순위

    # Claude 모델 이름을 로컬 Ollama 모델에 매핑
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

### Ollama 설정

1. [ollama.com](https://ollama.com)에서 Ollama 설치
2. 사용하려는 모델 풀:
   ```bash
   ollama pull qwen3:32b
   ollama pull qwen3:8b
   ollama pull codestral:latest
   ```
3. Ollama 시작 (설치 시 자동 실행)

### 권장 모델

Claude Code 워크플로우에는 최소 32K 컨텍스트를 가진 모델을 선택하세요:

| 모델 | 컨텍스트 | 크기 | 최적 용도 |
|------|---------|------|----------|
| `qwen3:32b` | 128K | 32B 파라미터 | 일반 코딩, 복잡한 추론 |
| `qwen3:8b` | 128K | 8B 파라미터 | 빠른 반복, 간단한 작업 |
| `codestral:latest` | 32K | 22B 파라미터 | 코드 생성, 전문 코딩 |
| `llama3.2:3b` | 128K | 3B 파라미터 | 매우 빠름, 기본 작업 |

### 기능 제한

Ollama의 Anthropic 호환성은 부분적입니다. 일부 기능은 지원되지 않습니다:

| 기능 | 지원 | 참고 |
|------|------|------|
| Streaming (SSE) | 예 | Anthropic과 동일한 이벤트 시퀀스 |
| Tool calling | 예 | Anthropic과 동일한 형식 |
| Extended thinking | 부분 | `budget_tokens` 허용되지만 적용되지 않음 |
| Prompt caching | 아니오 | `cache_control` 블록 무시됨 |
| PDF 입력 | 아니오 | 지원되지 않음 |
| 이미지 URL | 아니오 | Base64 인코딩만 지원 |
| 토큰 카운팅 | 아니오 | `/v1/messages/count_tokens` 사용 불가 |
| `tool_choice` | 아니오 | 특정 도구 사용 강제 불가 |

### Docker 네트워킹

Docker에서 cc-relay를 실행하고 호스트에서 Ollama를 실행할 때:

```yaml
providers:
  - name: "ollama"
    type: "ollama"
    # localhost 대신 Docker의 호스트 게이트웨이 사용
    base_url: "http://host.docker.internal:11434"
```

또는 `--network host`로 cc-relay 실행:

```bash
docker run --network host cc-relay
```

## AWS Bedrock 프로바이더

AWS Bedrock은 엔터프라이즈 보안과 SigV4 인증을 통해 Amazon Web Services를 통한 Claude 접근을 제공합니다.

```yaml
providers:
  - name: "bedrock"
    type: "bedrock"
    enabled: true
    aws_region: "us-east-1"
    model_mapping:
      "claude-sonnet-4-5-20250514": "anthropic.claude-sonnet-4-5-20250514-v1:0"
    keys:
      - key: "bedrock-internal"
```

Bedrock은 AWS SDK 표준 자격 증명 체인(환경 변수, IAM 역할 등)을 사용합니다.

## Azure AI Foundry 프로바이더

Azure AI Foundry는 엔터프라이즈 Azure 통합을 통해 Microsoft Azure를 통한 Claude 접근을 제공합니다.

```yaml
providers:
  - name: "azure"
    type: "azure"
    enabled: true
    azure_resource_name: "my-azure-resource"
    azure_api_version: "2024-06-01"
    keys:
      - key: "${AZURE_API_KEY}"
    model_mapping:
      "claude-sonnet-4-5-20250514": "claude-sonnet-4-5"
```

## Google Vertex AI 프로바이더

Vertex AI는 원활한 GCP 통합을 통해 Google Cloud를 통한 Claude 접근을 제공합니다.

```yaml
providers:
  - name: "vertex"
    type: "vertex"
    enabled: true
    gcp_project_id: "${GOOGLE_CLOUD_PROJECT}"
    gcp_region: "us-east5"
    model_mapping:
      "claude-sonnet-4-5-20250514": "claude-sonnet-4-5@20250514"
    keys:
      - key: "vertex-internal"
```

Vertex는 Google Application Default Credentials 또는 gcloud CLI를 사용합니다.

## Model Mapping

`model_mapping` 필드는 들어오는 모델 이름을 프로바이더별 모델로 변환합니다:

```yaml
providers:
  - name: "zai"
    type: "zai"
    model_mapping:
      # 형식: "들어오는-모델": "프로바이더-모델"
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-sonnet-4-5": "GLM-4.7"
```

Claude Code가 보낼 때:
```json
{"model": "claude-sonnet-4-5-20250514", ...}
```

CC-Relay는 Z.AI로 라우팅:
```json
{"model": "GLM-4.7", ...}
```

### 매핑 팁

1. **버전 접미사 포함**: `claude-sonnet-4-5`와 `claude-sonnet-4-5-20250514` 둘 다 매핑
2. **컨텍스트 길이 고려**: 유사한 기능을 가진 모델 매칭
3. **품질 테스트**: 출력 품질이 요구 사항에 맞는지 확인

## 멀티 프로바이더 설정

장애 조치, 비용 최적화 또는 부하 분산을 위해 여러 프로바이더를 설정합니다:

```yaml
providers:
  # 기본: Anthropic (최고 품질)
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # 먼저 시도

  # 보조: Z.AI (비용 효율적)
  - name: "zai"
    type: "zai"
    enabled: true
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # 폴백

  # 3차: Ollama (로컬, 무료)
  - name: "ollama"
    type: "ollama"
    enabled: true
    keys:
      - key: "ollama"
        priority: 0  # 최후의 수단

routing:
  strategy: failover  # 우선순위 순서로 프로바이더 시도
```

이 설정으로:
1. 요청이 먼저 Anthropic으로 (우선순위 2)
2. Anthropic 실패 시 (429, 5xx), Z.AI 시도 (우선순위 1)
3. Z.AI 실패 시, Ollama 시도 (우선순위 0)

더 많은 옵션은 [라우팅 전략](/ko/docs/routing/)을 참조하세요.

## 문제 해결

### 연결 거부 (Ollama)

**증상:** Ollama 연결 시 `connection refused`

**원인:**
- Ollama가 실행 중이 아님
- 잘못된 포트
- Docker 네트워킹 문제

**해결책:**
```bash
# Ollama 실행 중인지 확인
ollama list

# 포트 확인
curl http://localhost:11434/api/version

# Docker의 경우 호스트 게이트웨이 사용
base_url: "http://host.docker.internal:11434"
```

### 인증 실패 (Z.AI)

**증상:** Z.AI에서 `401 Unauthorized`

**원인:**
- 잘못된 API 키
- 환경 변수 미설정
- 키 미활성화

**해결책:**
```bash
# 환경 변수 확인
echo $ZAI_API_KEY

# 키 직접 테스트
curl -X POST https://api.z.ai/api/anthropic/v1/messages \
  -H "x-api-key: $ZAI_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"GLM-4.7","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### 모델을 찾을 수 없음

**증상:** `model not found` 오류

**원인:**
- 모델이 `models` 목록에 설정되지 않음
- `model_mapping` 항목 누락
- 모델 미설치 (Ollama)

**해결책:**
```yaml
# 모델이 목록에 있는지 확인
models:
  - "GLM-4.7"

# 매핑이 존재하는지 확인
model_mapping:
  "claude-sonnet-4-5": "GLM-4.7"
```

Ollama의 경우 모델이 설치되어 있는지 확인:
```bash
ollama list
ollama pull qwen3:32b
```

### 느린 응답 (Ollama)

**증상:** Ollama에서 매우 느린 응답

**원인:**
- 하드웨어에 비해 모델이 너무 큼
- GPU 미사용
- RAM 부족

**해결책:**
- 더 작은 모델 사용 (`qwen3:32b` 대신 `qwen3:8b`)
- GPU 활성화 확인: `ollama run qwen3:8b --verbose`
- 추론 중 메모리 사용량 확인

## 다음 단계

- [설정 참조](/ko/docs/configuration/) - 전체 설정 옵션
- [라우팅 전략](/ko/docs/routing/) - 프로바이더 선택 및 장애 조치
- [상태 모니터링](/ko/docs/health/) - 서킷 브레이커 및 상태 확인
