---
title: 설정
weight: 3
---

CC-Relay는 YAML 또는 TOML 파일로 설정됩니다. 이 가이드는 모든 설정 옵션을 다룹니다.

## 설정 파일 위치

기본 위치 (순서대로 확인):

1. `./config.yaml` 또는 `./config.toml` (현재 디렉토리)
2. `~/.config/cc-relay/config.yaml` 또는 `~/.config/cc-relay/config.toml`
3. `--config` 플래그로 지정된 경로

파일 확장자(`.yaml`, `.yml` 또는 `.toml`)에서 형식이 자동으로 감지됩니다。

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

# ==========================================================================
# 캐시 설정
# ==========================================================================
cache:
  # 캐시 모드: single, ha, disabled
  mode: single

  # 싱글 모드 (Ristretto) 설정
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size

  # HA 모드 (Olric) 설정
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

## 캐시 설정

CC-Relay는 다양한 배포 시나리오에 맞는 여러 백엔드 옵션을 지원하는 통합 캐시 레이어를 제공합니다.

### 캐시 모드

| 모드 | 백엔드 | 사용 사례 |
|------|---------|----------|
| `single` | [Ristretto](https://github.com/dgraph-io/ristretto) | 단일 인스턴스 배포, 고성능 |
| `ha` | [Olric](https://github.com/buraksezer/olric) | 다중 인스턴스 배포, 공유 상태 |
| `disabled` | Noop | 캐싱 없음, 패스스루 |

### 싱글 모드 (Ristretto)

Ristretto는 고성능 동시성 지원 인메모리 캐시입니다. 단일 인스턴스 배포의 기본 모드입니다.

```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size
```

| 필드 | 타입 | 기본값 | 설명 |
|------|------|--------|------|
| `num_counters` | int64 | 1,000,000 | 4비트 접근 카운터 수. 권장: 예상 최대 항목의 10배. |
| `max_cost` | int64 | 104,857,600 (100 MB) | 캐시가 보유할 수 있는 최대 메모리(바이트). |
| `buffer_items` | int64 | 64 | Get 버퍼당 키 수. 어드미션 버퍼 크기 제어. |

### HA 모드 (Olric) - 임베디드

공유 캐시 상태가 필요한 다중 인스턴스 배포의 경우, 각 cc-relay 인스턴스가 Olric 노드를 실행하는 임베디드 Olric 모드를 사용합니다.

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

| 필드 | 타입 | 기본값 | 설명 |
|------|------|--------|------|
| `embedded` | bool | false | 임베디드 Olric 노드 실행 (true) vs. 외부 클러스터 연결 (false). |
| `bind_addr` | string | 필수 | Olric 클라이언트 연결 주소 (예: "0.0.0.0:3320"). |
| `dmap_name` | string | "cc-relay" | 분산 맵 이름. 모든 노드가 동일한 이름을 사용해야 함. |
| `environment` | string | "local" | Memberlist 프리셋: "local", "lan", 또는 "wan". |
| `peers` | []string | - | 피어 검색을 위한 Memberlist 주소. bind_addr + 2 포트 사용. |
| `replica_count` | int | 1 | 키당 복제본 수. 1 = 복제 없음. |
| `read_quorum` | int | 1 | 응답에 필요한 최소 성공 읽기 수. |
| `write_quorum` | int | 1 | 응답에 필요한 최소 성공 쓰기 수. |
| `member_count_quorum` | int32 | 1 | 운영에 필요한 최소 클러스터 멤버 수. |
| `leave_timeout` | duration | 5s | 종료 전 이탈 메시지 브로드캐스트 시간. |

**중요:** Olric은 두 개의 포트를 사용합니다 - 클라이언트 연결용 `bind_addr` 포트와 memberlist 가십용 `bind_addr + 2`. 방화벽에서 두 포트 모두 열어야 합니다.

### HA 모드 (Olric) - 클라이언트 모드

임베디드 노드를 실행하는 대신 외부 Olric 클러스터에 연결합니다:

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

| 필드 | 타입 | 설명 |
|------|------|------|
| `embedded` | bool | 클라이언트 모드에서는 `false`로 설정. |
| `addresses` | []string | 외부 Olric 클러스터 주소. |
| `dmap_name` | string | 분산 맵 이름 (클러스터 설정과 일치해야 함). |

### 비활성화 모드

디버깅용이거나 다른 곳에서 캐싱을 처리할 때 캐싱을 완전히 비활성화합니다:

```yaml
cache:
  mode: disabled
```

HA 클러스터링 가이드 및 문제 해결을 포함한 전체 캐시 문서는 [캐싱](/ko/docs/caching/)을 참조하세요.

## 라우팅 설정

CC-Relay는 프로바이더 간에 요청을 분배하기 위한 여러 라우팅 전략을 지원합니다.

```yaml
# ==========================================================================
# 라우팅 설정
# ==========================================================================
routing:
  # 전략: round_robin, weighted_round_robin, shuffle, failover (기본값)
  strategy: failover

  # 장애 조치 시도의 타임아웃 (밀리초, 기본값: 5000)
  failover_timeout: 5000

  # 디버그 헤더 활성화 (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false
```

### 라우팅 전략

| 전략 | 설명 |
|------|------|
| `failover` | 우선순위 순서로 프로바이더 시도, 실패 시 대체 (기본값) |
| `round_robin` | 프로바이더 간 순차 로테이션 |
| `weighted_round_robin` | 가중치에 따른 비례 분배 |
| `shuffle` | 공정한 무작위 분배 |

### 프로바이더 가중치 및 우선순위

가중치와 우선순위는 프로바이더의 첫 번째 키에서 설정합니다:

```yaml
providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3      # weighted-round-robin용 (높을수록 = 더 많은 트래픽)
        priority: 2    # failover용 (높을수록 = 먼저 시도)
```

전략 설명, 디버그 헤더, 장애 조치 트리거를 포함한 자세한 라우팅 설정은 [라우팅](/ko/docs/routing/)을 참조하세요.

## 설정 예제

### 최소 단일 프로바이더

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

### 멀티 프로바이더 설정

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

### 디버그 로깅을 포함한 개발 설정

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

## 설정 검증

설정 파일을 검증합니다:

```bash
cc-relay config validate
```

**팁**: 배포 전에 항상 설정 변경 사항을 검증하세요. 핫 리로드는 유효하지 않은 설정을 거부하지만, 검증은 프로덕션에 도달하기 전에 오류를 감지합니다.

## 핫 리로딩

CC-Relay는 재시작 없이 설정 변경을 자동으로 감지하고 적용합니다. 이를 통해 다운타임 없이 설정을 업데이트할 수 있습니다.

### 작동 방식

CC-Relay는 [fsnotify](https://github.com/fsnotify/fsnotify)를 사용하여 설정 파일을 모니터링합니다:

1. **파일 모니터링**: 상위 디렉토리를 모니터링하여 원자적 쓰기(대부분의 편집기에서 사용하는 임시 파일 + 이름 변경 패턴)를 올바르게 감지
2. **디바운싱**: 여러 빠른 파일 이벤트는 100ms 지연으로 통합하여 편집기 저장 동작을 처리
3. **원자적 스왑**: 새 설정은 Go의 `sync/atomic.Pointer`를 사용하여 원자적으로 로드 및 스왑
4. **진행 중인 요청 보존**: 진행 중인 요청은 이전 설정을 계속 사용하고, 새 요청은 업데이트된 설정을 사용

### 리로드를 트리거하는 이벤트

| 이벤트 | 리로드 트리거 |
|--------|--------------|
| 파일 쓰기 | 예 |
| 파일 생성 (원자적 이름 변경) | 예 |
| 파일 chmod | 아니오 (무시) |
| 디렉토리 내 다른 파일 | 아니오 (무시) |

### 로깅

핫 리로드 발생 시 로그 메시지가 표시됩니다:

```
INF config file reloaded path=/path/to/config.yaml
INF config hot-reloaded successfully
```

새 설정이 유효하지 않은 경우:

```
ERR failed to reload config path=/path/to/config.yaml error="validation error"
```

유효하지 않은 설정은 거부되고 프록시는 이전의 유효한 설정으로 계속 실행됩니다.

### 제한 사항

- **프로바이더 변경**: 프로바이더 추가 또는 제거는 재시작 필요 (라우팅 인프라는 시작 시 초기화됩니다)
- **리슨 주소**: `server.listen` 변경은 재시작 필요
- **gRPC 주소**: gRPC 관리 API 주소 변경은 재시작 필요

핫 리로드 가능한 설정 옵션:
- 로그 레벨 및 형식
- 기존 키의 속도 제한
- 헬스 체크 간격
- 라우팅 전략 가중치 및 우선순위

## 다음 단계

- [라우팅 전략](/ko/docs/routing/) - 프로바이더 선택 및 장애 조치
- [아키텍처 이해](/ko/docs/architecture/)
- [API 레퍼런스](/ko/docs/api/)
