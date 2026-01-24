---
title: 라우팅
weight: 4
---

CC-Relay는 프로바이더 간에 요청을 분배하기 위한 여러 라우팅 전략을 지원합니다. 이 페이지에서는 각 전략과 설정 방법을 설명합니다.

## 개요

라우팅은 cc-relay가 어떤 프로바이더가 각 요청을 처리할지 결정하는 방법입니다. 적합한 전략은 가용성, 비용, 지연 시간 또는 부하 분산과 같은 우선순위에 따라 달라집니다.

| 전략 | 설정 값 | 설명 | 사용 사례 |
|------|---------|------|----------|
| Round-Robin | `round_robin` | 프로바이더 간 순차 로테이션 | 균등 분배 |
| Weighted Round-Robin | `weighted_round_robin` | 가중치에 따른 비례 분배 | 용량 기반 분배 |
| Shuffle | `shuffle` | 공정한 무작위 ("카드 배분" 패턴) | 무작위 부하 분산 |
| Failover | `failover` (기본값) | 우선순위 기반 자동 재시도 | 고가용성 |
| Model-Based | `model_based` | 모델 이름 접두사로 라우팅 | 다중 모델 배포 |

## 설정

`config.yaml`에서 라우팅을 설정합니다:

```yaml
routing:
  # 전략: round_robin, weighted_round_robin, shuffle, failover (기본값), model_based
  strategy: failover

  # 장애 조치 시도의 타임아웃 (밀리초, 기본값: 5000)
  failover_timeout: 5000

  # 디버그 헤더 활성화 (X-CC-Relay-Strategy, X-CC-Relay-Provider)
  debug: false

  # 모델 기반 라우팅 설정 (strategy: model_based인 경우에만 사용)
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    glm-4: zai
    qwen: ollama
  default_provider: anthropic
```

**기본값:** `strategy`가 지정되지 않으면 cc-relay는 가장 안전한 옵션인 `failover`를 사용합니다.

## 전략

### Round-Robin

원자적 카운터를 사용한 순차 분배. 어떤 프로바이더도 두 번째 요청을 받기 전에 각 프로바이더가 하나의 요청을 받습니다.

```yaml
routing:
  strategy: round_robin
```

**작동 원리:**

1. 요청 1 → 프로바이더 A
2. 요청 2 → 프로바이더 B
3. 요청 3 → 프로바이더 C
4. 요청 4 → 프로바이더 A (사이클 반복)

**최적의 용도:** 유사한 용량을 가진 프로바이더 간 균등 분배.

### Weighted Round-Robin

프로바이더 가중치에 따라 요청을 비례 분배합니다. 균등한 분배를 위해 Nginx smooth weighted round-robin 알고리즘을 사용합니다.

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        weight: 3  # 3배의 요청을 받음

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        weight: 1  # 1배의 요청을 받음
```

**작동 원리:**

가중치가 3:1인 경우, 4개 요청마다:
- 3개 요청 → anthropic
- 1개 요청 → zai

**기본 가중치:** 1 (지정되지 않은 경우)

**최적의 용도:** 프로바이더 용량, 속도 제한 또는 비용 할당에 따른 부하 분배.

### Shuffle

Fisher-Yates "카드 배분" 패턴을 사용한 공정한 무작위 분배. 누군가 두 번째 카드를 받기 전에 모든 사람이 한 장씩 받습니다.

```yaml
routing:
  strategy: shuffle
```

**작동 원리:**

1. 모든 프로바이더가 "덱"에 들어감
2. 무작위 프로바이더가 선택되어 덱에서 제거됨
3. 덱이 비면 모든 프로바이더를 다시 셔플
4. 시간이 지나도 공정한 분배를 보장

**최적의 용도:** 공정성을 보장하면서 무작위 부하 분산.

### Failover

우선순위 순서로 프로바이더를 시도합니다. 실패 시 가장 빠른 성공 응답을 위해 나머지 프로바이더에 병렬 요청을 실행합니다. 이것이 **기본 전략**입니다.

```yaml
routing:
  strategy: failover

providers:
  - name: "anthropic"
    type: "anthropic"
    keys:
      - key: "${ANTHROPIC_API_KEY}"
        priority: 2  # 먼저 시도됨 (높을수록 = 높은 우선순위)

  - name: "zai"
    type: "zai"
    keys:
      - key: "${ZAI_API_KEY}"
        priority: 1  # 대체
```

**작동 원리:**

1. 가장 높은 우선순위의 프로바이더를 먼저 시도
2. 실패하면 ([장애 조치 트리거](#장애-조치-트리거) 참조) 나머지 모든 프로바이더에 병렬 요청 발행
3. 첫 번째 성공 응답을 반환하고 나머지는 취소
4. 전체 작업 시간에 대해 `failover_timeout`을 준수

**기본 우선순위:** 1 (지정되지 않은 경우)

**최적의 용도:** 자동 장애 조치가 있는 고가용성.

### Model-Based

요청의 모델 이름을 기반으로 프로바이더에 요청을 라우팅합니다. 특이성을 위해 최장 접두사 매칭을 사용합니다.

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

**작동 원리:**

1. 요청에서 `model` 매개변수를 추출
2. `model_mapping`에서 최장 접두사 일치를 찾음
3. 해당 프로바이더로 라우팅
4. 일치하는 항목이 없으면 `default_provider`로 폴백
5. 일치 항목도 기본값도 없으면 오류 반환

**접두사 매칭 예제:**

| 요청된 모델 | 매핑 항목 | 선택된 항목 | 프로바이더 |
|-----------|----------|-----------|----------|
| `claude-opus-4` | `claude-opus`, `claude` | `claude-opus` | anthropic |
| `claude-sonnet-3.5` | `claude-sonnet`, `claude` | `claude-sonnet` | anthropic |
| `glm-4-plus` | `glm-4`, `glm` | `glm-4` | zai |
| `qwen-72b` | `qwen`, `claude` | `qwen` | ollama |
| `llama-3.2` | `llama`, `claude` | `llama` | ollama |
| `gpt-4` | `claude`, `llama` | (일치 없음) | default_provider |

**최적의 용도:** 다른 모델을 다른 프로바이더로 라우팅해야 하는 다중 모델 배포.

## 디버그 헤더

`routing.debug: true`인 경우 cc-relay는 응답에 진단 헤더를 추가합니다:

| 헤더 | 값 | 설명 |
|------|-----|------|
| `X-CC-Relay-Strategy` | 전략 이름 | 사용된 라우팅 전략 |
| `X-CC-Relay-Provider` | 프로바이더 이름 | 요청을 처리한 프로바이더 |

**응답 헤더 예시:**

```
X-CC-Relay-Strategy: failover
X-CC-Relay-Provider: anthropic
```

**보안 경고:** 디버그 헤더는 내부 라우팅 결정을 노출합니다. 개발 환경 또는 신뢰할 수 있는 환경에서만 사용하세요. 신뢰할 수 없는 클라이언트가 있는 프로덕션 환경에서는 절대 활성화하지 마세요.

## 장애 조치 트리거

failover 전략은 특정 오류 조건에서 재시도를 트리거합니다:

| 트리거 | 조건 | 설명 |
|--------|------|------|
| 상태 코드 | `429`, `500`, `502`, `503`, `504` | 속도 제한 또는 서버 오류 |
| 타임아웃 | `context.DeadlineExceeded` | 요청 타임아웃 초과 |
| 연결 | `net.Error` | 네트워크 오류, DNS 실패, 연결 거부 |

**중요:** 클라이언트 오류 (429를 제외한 4xx)는 장애 조치를 트리거**하지 않습니다**. 이는 프로바이더가 아닌 요청 자체의 문제를 나타냅니다.

### 상태 코드 설명

| 코드 | 의미 | 장애 조치? |
|------|------|-----------|
| `429` | 속도 제한 | 예 - 다른 프로바이더 시도 |
| `500` | Internal Server Error | 예 - 서버 문제 |
| `502` | Bad Gateway | 예 - 업스트림 문제 |
| `503` | Service Unavailable | 예 - 일시적 다운 |
| `504` | Gateway Timeout | 예 - 업스트림 타임아웃 |
| `400` | Bad Request | 아니오 - 요청 수정 |
| `401` | Unauthorized | 아니오 - 인증 수정 |
| `403` | Forbidden | 아니오 - 권한 문제 |

## 예제

### 간단한 Failover (대부분의 사용자에게 권장)

우선순위가 지정된 프로바이더로 기본 전략 사용:

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

### 가중치 부하 분산

프로바이더 용량에 따라 부하 분배:

```yaml
routing:
  strategy: weighted_round_robin

providers:
  - name: "primary"
    type: "anthropic"
    keys:
      - key: "${PRIMARY_KEY}"
        weight: 3  # 트래픽의 75%

  - name: "secondary"
    type: "anthropic"
    keys:
      - key: "${SECONDARY_KEY}"
        weight: 1  # 트래픽의 25%
```

### 디버그 헤더를 포함한 개발 환경

문제 해결을 위해 디버그 헤더 활성화:

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

### 빠른 장애 조치로 고가용성

장애 조치 지연 시간 최소화:

```yaml
routing:
  strategy: failover
  failover_timeout: 3000  # 3초 타임아웃

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

### 모델 기반 라우팅을 사용한 다중 모델

다른 모델을 전용 프로바이더로 라우팅:

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

이 설정으로:
- Claude 모델 → Anthropic
- GLM 모델 → Z.AI
- Qwen/Llama 모델 → Ollama (로컬)
- 기타 모델 → Anthropic (기본값)

## 프로바이더 가중치 및 우선순위

가중치와 우선순위는 프로바이더의 키 설정에서 지정합니다:

```yaml
providers:
  - name: "example"
    type: "anthropic"
    keys:
      - key: "${API_KEY}"
        weight: 3      # weighted-round-robin용 (높을수록 = 더 많은 트래픽)
        priority: 2    # failover용 (높을수록 = 먼저 시도)
        rpm_limit: 60  # 속도 제한 추적
```

**참고:** 가중치와 우선순위는 프로바이더 키 목록의 **첫 번째 키**에서 읽습니다.

## 다음 단계

- [설정 레퍼런스](/ko/docs/configuration/) - 전체 설정 옵션
- [아키텍처 개요](/ko/docs/architecture/) - cc-relay 내부 동작 방식
