---
title: 시작하기
weight: 2
---

이 가이드는 CC-Relay 설치, 설정 및 첫 실행 과정을 안내합니다.

## 사전 요구 사항

- **Go 1.21+** 소스에서 빌드 시 필요
- **API 키** 지원되는 프로바이더(Anthropic 또는 Z.AI) 중 하나 이상
- **Claude Code** CLI 테스트용 (선택 사항)

## 설치

### Go Install 사용

```bash
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest
```

바이너리는 `$GOPATH/bin/cc-relay` 또는 `$HOME/go/bin/cc-relay`에 설치됩니다.

### 소스에서 빌드

```bash
# 저장소 클론
git clone https://github.com/omarluq/cc-relay.git
cd cc-relay

# task로 빌드 (권장)
task build

# 또는 수동 빌드
go build -o cc-relay ./cmd/cc-relay

# 실행
./cc-relay --help
```

### 미리 빌드된 바이너리

[릴리스 페이지](https://github.com/omarluq/cc-relay/releases)에서 미리 빌드된 바이너리를 다운로드하세요.

## 빠른 시작

### 1. 설정 초기화

CC-Relay는 기본 설정 파일을 자동으로 생성할 수 있습니다:

```bash
cc-relay config init
```

이 명령은 `~/.config/cc-relay/config.yaml`에 기본값이 설정된 설정 파일을 생성합니다.

### 2. 환경 변수 설정

```bash
export ANTHROPIC_API_KEY="your-api-key-here"

# 선택 사항: Z.AI 사용 시
export ZAI_API_KEY="your-zai-key-here"
```

### 3. CC-Relay 실행

```bash
cc-relay serve
```

다음과 같은 출력이 표시됩니다:

```
INF starting cc-relay listen=127.0.0.1:8787
INF using primary provider provider=anthropic-pool type=anthropic
```

### 4. Claude Code 설정

CC-Relay를 사용하도록 Claude Code를 설정하는 가장 쉬운 방법:

```bash
cc-relay config cc init
```

이 명령은 `~/.claude/settings.json`을 프록시 설정으로 자동 업데이트합니다.

또는 환경 변수를 수동으로 설정할 수 있습니다:

```bash
export ANTHROPIC_BASE_URL="http://localhost:8787"
export ANTHROPIC_AUTH_TOKEN="managed-by-cc-relay"
claude
```

## 작동 확인

### 서버 상태 확인

```bash
cc-relay status
```

출력:
```
✓ cc-relay is running (127.0.0.1:8787)
```

### Health 엔드포인트 테스트

```bash
curl http://localhost:8787/health
```

응답:
```json
{"status":"ok"}
```

### 사용 가능한 모델 목록

```bash
curl http://localhost:8787/v1/models
```

### 요청 테스트

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

## CLI 명령어

CC-Relay는 다음과 같은 CLI 명령어를 제공합니다:

| 명령어 | 설명 |
|---------|-------------|
| `cc-relay serve` | 프록시 서버 시작 |
| `cc-relay status` | 서버 실행 여부 확인 |
| `cc-relay config init` | 기본 설정 파일 생성 |
| `cc-relay config cc init` | Claude Code가 cc-relay를 사용하도록 설정 |
| `cc-relay config cc remove` | Claude Code에서 cc-relay 설정 제거 |
| `cc-relay --version` | 버전 정보 표시 |

### Serve 명령어 옵션

```bash
cc-relay serve [flags]

Flags:
  --config string      설정 파일 경로 (기본값: ~/.config/cc-relay/config.yaml)
  --log-level string   로그 레벨 (debug, info, warn, error)
  --log-format string  로그 형식 (json, text)
  --debug              디버그 모드 활성화 (상세 로깅)
```

## 최소 설정

다음은 최소한으로 작동하는 설정입니다:

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

## 다음 단계

- [여러 프로바이더 설정](/ko/docs/configuration/)
- [아키텍처 이해](/ko/docs/architecture/)
- [API 레퍼런스](/ko/docs/api/)

## 문제 해결

### 포트가 이미 사용 중

8787 포트가 이미 사용 중이라면 설정에서 listen 주소를 변경하세요:

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

### 프로바이더가 응답하지 않음

서버 로그에서 연결 오류를 확인하세요:

```bash
cc-relay serve --log-level debug
```

### 인증 오류

"authentication failed" 오류가 표시되면:

1. 환경 변수에 API 키가 올바르게 설정되었는지 확인
2. 설정 파일이 올바른 환경 변수를 참조하는지 확인
3. 프로바이더에서 API 키가 유효한지 확인

### 디버그 모드

상세한 요청/응답 로깅을 위해 디버그 모드를 활성화하세요:

```bash
cc-relay serve --debug
```

이 모드는 다음을 활성화합니다:
- 디버그 로그 레벨
- 요청 본문 로깅 (민감한 필드는 마스킹)
- 응답 헤더 로깅
- TLS 연결 메트릭
