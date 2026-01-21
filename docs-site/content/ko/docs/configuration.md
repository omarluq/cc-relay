---
title: 설정
weight: 3
---

CC-Relay는 YAML 파일로 설정됩니다. 이 가이드는 모든 설정 옵션을 다룹니다.

## 설정 파일 위치

기본 위치 (순서대로 확인):

1. `./config.yaml` (현재 디렉토리)
2. `~/.config/cc-relay/config.yaml`
3. `--config` 플래그로 지정된 경로

다음 명령으로 기본 설정을 생성하세요:

```bash
cc-relay config init
```

## 환경 변수 확장

CC-Relay는 `${VAR_NAME}` 구문을 사용한 환경 변수 확장을 지원합니다:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"  # 로드 시 확장됨
```

## 전체 설정 레퍼런스

```yaml
# ==========================================================================
# 서버 설정
# ==========================================================================
server:
  # 수신 대기 주소
  listen: "127.0.0.1:8787"

  # 요청 타임아웃 (밀리초, 기본값: 600000 = 10분)
  timeout_ms: 600000

  # 최대 동시 요청 수 (0 = 무제한)
  max_concurrent: 0

  # 성능 향상을 위해 HTTP/2 활성화
  enable_http2: true

  # 인증 설정
  auth:
    # 프록시 접근에 특정 API 키 요구
    api_key: "${PROXY_API_KEY}"

    # Claude Code 구독 Bearer 토큰 허용
    allow_subscription: true

    # 검증할 특정 Bearer 토큰 (선택 사항)
    bearer_secret: "${BEARER_SECRET}"

# ==========================================================================
# 프로바이더 설정
# ==========================================================================
providers:
  # Anthropic 직접 API
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # 선택 사항, 기본값 사용

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60       # 분당 요청 수
        tpm_limit: 100000   # 분당 토큰 수

    # 선택 사항: 사용 가능한 모델 지정
    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"

  # Z.AI / Zhipu GLM
  - name: "zai"
    type: "zai"
    enabled: true
    base_url: "https://api.z.ai/api/anthropic"

    keys:
      - key: "${ZAI_API_KEY}"

    # Claude 모델명을 Z.AI 모델로 매핑
    model_mapping:
      "claude-sonnet-4-5-20250514": "GLM-4.7"
      "claude-haiku-3-5-20241022": "GLM-4.5-Air"

    # 선택 사항: 사용 가능한 모델 지정
    models:
      - "GLM-4.7"
      - "GLM-4.5-Air"
      - "GLM-4-Plus"

# ==========================================================================
# 로깅 설정
# ==========================================================================
logging:
  # 로그 레벨: debug, info, warn, error
  level: "info"

  # 로그 형식: json, text
  format: "text"

  # 컬러 출력 활성화 (text 형식용)
  pretty: true

  # 상세 디버그 옵션
  debug_options:
    log_request_body: false
    log_response_headers: false
    log_tls_metrics: false
    max_body_log_size: 1000
```

## 서버 설정

### 수신 대기 주소

`listen` 필드는 프록시가 요청을 수신하는 위치를 지정합니다:

```yaml
server:
  listen: "127.0.0.1:8787"  # 로컬 전용 (권장)
  # listen: "0.0.0.0:8787"  # 모든 인터페이스 (주의해서 사용)
```

### 인증

CC-Relay는 여러 인증 방식을 지원합니다:

#### API 키 인증

클라이언트에게 특정 API 키를 요구합니다:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
```

클라이언트는 헤더를 포함해야 합니다: `x-api-key: <your-proxy-key>`

#### Claude Code 구독 패스스루

Claude Code 구독 사용자 연결을 허용합니다:

```yaml
server:
  auth:
    allow_subscription: true
```

Claude Code의 `Authorization: Bearer` 토큰을 수락합니다.

#### 복합 인증

API 키와 구독 인증 모두 허용:

```yaml
server:
  auth:
    api_key: "${PROXY_API_KEY}"
    allow_subscription: true
```

#### 인증 없음

인증을 비활성화하려면 (프로덕션에서는 권장하지 않음):

```yaml
server:
  auth: {}
  # 또는 단순히 auth 섹션 생략
```

### HTTP/2 지원

동시 요청 성능 향상을 위해 HTTP/2를 활성화합니다:

```yaml
server:
  enable_http2: true
```

## 프로바이더 설정

### 프로바이더 유형

CC-Relay는 현재 두 가지 프로바이더 유형을 지원합니다:

| 유형 | 설명 | 기본 Base URL |
|------|-------------|------------------|
| `anthropic` | Anthropic 직접 API | `https://api.anthropic.com` |
| `zai` | Z.AI / Zhipu GLM | `https://api.z.ai/api/anthropic` |

### Anthropic 프로바이더

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    enabled: true
    base_url: "https://api.anthropic.com"  # 선택 사항

    keys:
      - key: "${ANTHROPIC_API_KEY}"
        rpm_limit: 60
        tpm_limit: 100000

    models:
      - "claude-sonnet-4-5-20250514"
      - "claude-opus-4-5-20250514"
      - "claude-haiku-3-5-20241022"
```

### Z.AI 프로바이더

Z.AI는 저렴한 비용으로 GLM 모델과 함께 Anthropic 호환 API를 제공합니다:

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

### 다중 API 키

처리량 향상을 위해 여러 API 키를 풀링합니다:

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

### 커스텀 Base URL

기본 API 엔드포인트를 재정의합니다:

```yaml
providers:
  - name: "anthropic-custom"
    type: "anthropic"
    base_url: "https://custom-endpoint.example.com"
```

## 로깅 설정

### 로그 레벨

| 레벨 | 설명 |
|-------|-------------|
| `debug` | 개발용 상세 출력 |
| `info` | 정상 작동 메시지 |
| `warn` | 경고 메시지 |
| `error` | 오류 메시지만 |

### 로그 형식

```yaml
logging:
  format: "text"   # 사람이 읽기 쉬운 형식 (기본값)
  # format: "json" # 기계가 읽기 쉬운 형식, 로그 집계용
```

### 디버그 옵션

디버그 로깅에 대한 세밀한 제어:

```yaml
logging:
  level: "debug"
  debug_options:
    log_request_body: true      # 요청 본문 로깅 (마스킹됨)
    log_response_headers: true  # 응답 헤더 로깅
    log_tls_metrics: true       # TLS 연결 정보 로깅
    max_body_log_size: 1000     # 본문에서 로깅할 최대 바이트
```

## 설정 예제

### 최소 단일 프로바이더

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

### 멀티 프로바이더 설정

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

### 디버그 로깅을 포함한 개발 설정

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

## 설정 검증

설정 파일을 검증합니다:

```bash
cc-relay config validate
```

## 핫 리로딩

설정 변경 시 서버를 재시작해야 합니다. 핫 리로딩은 향후 릴리스에 계획되어 있습니다.

## 다음 단계

- [아키텍처 이해](/ko/docs/architecture/)
- [API 레퍼런스](/ko/docs/api/)
