---
title: 문서
weight: 1
---

CC-Relay 문서에 오신 것을 환영합니다! 이 가이드는 Claude Code 및 기타 LLM 클라이언트를 위한 멀티 프로바이더 프록시로 CC-Relay를 설정, 구성 및 사용하는 데 도움을 드립니다.

## CC-Relay란?

CC-Relay는 Go로 작성된 고성능 HTTP 프록시로, LLM 클라이언트(Claude Code 등)와 LLM 프로바이더 사이에 위치합니다. 다음을 제공합니다:

- **멀티 프로바이더 지원**: Anthropic 및 Z.AI (더 많은 프로바이더 지원 예정)
- **Anthropic API 호환**: 직접 API 접근을 대체하는 드롭인 솔루션
- **SSE 스트리밍**: 스트리밍 응답 완벽 지원
- **다중 인증 방식**: API 키 및 Bearer 토큰 지원
- **Claude Code 통합**: 내장 설정 명령어로 간편한 설정

## 현재 상태

CC-Relay는 활발하게 개발 중입니다. 현재 구현된 기능:

| 기능 | 상태 |
|---------|--------|
| HTTP 프록시 서버 | 구현됨 |
| Anthropic 프로바이더 | 구현됨 |
| Z.AI 프로바이더 | 구현됨 |
| SSE 스트리밍 | 구현됨 |
| API 키 인증 | 구현됨 |
| Bearer 토큰 (구독) 인증 | 구현됨 |
| Claude Code 설정 | 구현됨 |
| 다중 API 키 | 구현됨 |
| 디버그 로깅 | 구현됨 |

**예정된 기능:**
- 라우팅 전략 (라운드 로빈, 페일오버, 비용 기반)
- API 키별 요청 제한
- 서킷 브레이커 및 상태 추적
- gRPC 관리 API
- TUI 대시보드
- 추가 프로바이더 (Ollama, Bedrock, Azure, Vertex)

## 빠른 시작

```bash
# 설치
go install github.com/omarluq/cc-relay/cmd/cc-relay@latest

# 설정 초기화
cc-relay config init

# API 키 설정
export ANTHROPIC_API_KEY="your-key-here"

# 프록시 시작
cc-relay serve

# Claude Code 설정 (다른 터미널에서)
cc-relay config cc init
```

## 빠른 탐색

- [시작하기](/ko/docs/getting-started/) - 설치 및 첫 실행
- [설정](/ko/docs/configuration/) - 프로바이더 설정 및 옵션
- [아키텍처](/ko/docs/architecture/) - 시스템 설계 및 컴포넌트
- [API 레퍼런스](/ko/docs/api/) - HTTP 엔드포인트 및 예제

## 문서 섹션

### 시작하기
- [설치](/ko/docs/getting-started/#설치)
- [빠른 시작](/ko/docs/getting-started/#빠른-시작)
- [CLI 명령어](/ko/docs/getting-started/#cli-명령어)
- [Claude Code로 테스트](/ko/docs/getting-started/#claude-code로-테스트)
- [문제 해결](/ko/docs/getting-started/#문제-해결)

### 설정
- [서버 설정](/ko/docs/configuration/#서버-설정)
- [프로바이더 설정](/ko/docs/configuration/#프로바이더-설정)
- [인증](/ko/docs/configuration/#인증)
- [로깅 설정](/ko/docs/configuration/#로깅-설정)
- [설정 예제](/ko/docs/configuration/#설정-예제)

### 아키텍처
- [시스템 개요](/ko/docs/architecture/#시스템-개요)
- [핵심 컴포넌트](/ko/docs/architecture/#핵심-컴포넌트)
- [요청 흐름](/ko/docs/architecture/#요청-흐름)
- [SSE 스트리밍](/ko/docs/architecture/#sse-스트리밍)
- [인증 흐름](/ko/docs/architecture/#인증-흐름)

### API 레퍼런스
- [POST /v1/messages](/ko/docs/api/#post-v1messages)
- [GET /v1/models](/ko/docs/api/#get-v1models)
- [GET /v1/providers](/ko/docs/api/#get-v1providers)
- [GET /health](/ko/docs/api/#get-health)
- [클라이언트 예제](/ko/docs/api/#curl-예제)

## 도움이 필요하신가요?

- [이슈 신고](https://github.com/omarluq/cc-relay/issues)
- [토론](https://github.com/omarluq/cc-relay/discussions)
