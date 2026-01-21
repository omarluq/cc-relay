---
title: 소개
type: about
---

## CC-Relay 소개

CC-Relay는 Go로 작성된 고성능 HTTP 프록시로, Claude Code 및 기타 LLM 클라이언트가 단일 엔드포인트를 통해 여러 프로바이더에 연결할 수 있게 해줍니다.

### 프로젝트 목표

- **멀티 프로바이더 접근 간소화** - 하나의 프록시로 여러 백엔드 연결
- **API 호환성 유지** - Anthropic API 직접 접근을 대체하는 드롭인 솔루션
- **유연성 제공** - 클라이언트 변경 없이 손쉬운 프로바이더 전환
- **Claude Code 지원** - Claude Code CLI와의 최상의 통합

### 현재 상태

CC-Relay는 활발하게 개발 중입니다. 다음 기능들이 구현되어 있습니다:

- Anthropic API 호환 HTTP 프록시 서버
- Anthropic 및 Z.AI 프로바이더 지원
- 완전한 SSE 스트리밍 지원
- API 키 및 Bearer 토큰 인증
- 프로바이더당 다중 API 키
- 요청/응답 검사를 위한 디버그 로깅
- Claude Code 설정 명령어

### 예정된 기능

- 추가 프로바이더 (Ollama, AWS Bedrock, Azure, Vertex AI)
- 라우팅 전략 (라운드 로빈, 페일오버, 비용 기반)
- API 키별 요청 제한
- 서킷 브레이커 및 상태 추적
- gRPC 관리 API
- TUI 대시보드

### 사용 기술

- [Go](https://go.dev/) - 프로그래밍 언어
- [Cobra](https://cobra.dev/) - CLI 프레임워크
- [zerolog](https://github.com/rs/zerolog) - 구조화된 로깅

### 저자

[Omar Alani](https://github.com/omarluq)가 개발했습니다.

### 라이선스

CC-Relay는 [AGPL 3 라이선스](https://github.com/omarluq/cc-relay/blob/main/LICENSE) 하에 배포되는 오픈 소스 소프트웨어입니다.

### 기여하기

기여를 환영합니다! 다음 링크에서 [GitHub 저장소](https://github.com/omarluq/cc-relay)를 확인하세요:

- [이슈 신고](https://github.com/omarluq/cc-relay/issues)
- [풀 리퀘스트 제출](https://github.com/omarluq/cc-relay/pulls)
- [토론](https://github.com/omarluq/cc-relay/discussions)
