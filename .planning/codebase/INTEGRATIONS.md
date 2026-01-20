# External Integrations

**Analysis Date:** 2026-01-20

## APIs & External Services

### LLM Providers

**Anthropic (Claude API):**
- What it's used for: Primary LLM provider, native Anthropic API compatibility
- Base URL: `https://api.anthropic.com`
- SDK/Client: None (uses standard HTTP client)
- Auth: `x-api-key` header
- Environment variable: `ANTHROPIC_API_KEY`
- Features: Full support including extended thinking, prompt caching, vision
- Rate limits: RPM and TPM tracked per key

**Z.AI / Zhipu GLM:**
- What it's used for: Cost-effective alternative provider (~1/7 cost of Anthropic)
- Base URL: `https://api.z.ai/api/anthropic`
- SDK/Client: None (uses standard HTTP client)
- Auth: `ANTHROPIC_AUTH_TOKEN` header (Anthropic-compatible)
- Environment variable: `ZAI_API_KEY`
- Features: Full Anthropic API compatibility with GLM-4.7 and GLM-4.5-Air models
- Model mapping: Transforms Anthropic model names to GLM model identifiers

**Ollama (Local LLM):**
- What it's used for: Local model inference, offline capability
- Base URL: `http://localhost:11434`
- SDK/Client: None (uses standard HTTP client)
- Auth: None required
- Features: Limited feature support (no prompt caching, no extended thinking, no PDF vision)
- Model mapping: Maps Anthropic model names to local Ollama models
- Image handling: Requires base64 encoding (no URL support)

**AWS Bedrock:**
- What it's used for: Enterprise-grade AWS-hosted Claude models
- Base URL: `bedrock-runtime.{region}.amazonaws.com`
- SDK/Client: AWS SDK for Go (planned: `github.com/aws/aws-sdk-go`)
- Auth: AWS SigV4 signing or Bearer Token (`AWS_BEARER_TOKEN_BEDROCK` - new July 2025 feature)
- Model format: `anthropic.claude-sonnet-4-5-20250929-v1:0` in URL path
- Region: Configurable in provider config (e.g., `us-east-1`)
- API header: `anthropic_version: "bedrock-2023-05-31"`
- Environment variables: AWS credentials via standard AWS_* vars or `AWS_BEARER_TOKEN_BEDROCK`

**Azure AI Foundry:**
- What it's used for: Azure-hosted Claude models with enterprise integration
- Base URL: `{resource}.services.ai.azure.com/anthropic`
- SDK/Client: None (uses standard HTTP client with custom auth)
- Auth methods:
  - API Key: `x-api-key` header with `${AZURE_API_KEY}`
  - Entra ID (Azure AD): Tenant ID, Client ID, Client Secret
- Model mapping: Azure deployment names map to Anthropic model identifiers
- Environment variables: `AZURE_API_KEY`, `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`
- Resource: Configurable in provider config

**Google Vertex AI:**
- What it's used for: Google Cloud-hosted Claude models
- Base URL: `{region}-aiplatform.googleapis.com`
- SDK/Client: Google Cloud Auth libraries (planned: `google.golang.org/api`)
- Auth: Google OAuth via service account
- Model format: `claude-sonnet-4-5@20250929` in URL path
- API header: `anthropic_version: "vertex-2023-10-16"`
- Environment variables: `GOOGLE_APPLICATION_CREDENTIALS` (service account JSON path), `GOOGLE_CLOUD_PROJECT`
- Region: Configurable (e.g., `us-east5`, `global`)

## Data Storage

**Databases:**
- None required for core operation

**File Storage:**
- Local filesystem only
  - Configuration: `~/.config/cc-relay/config.yaml`
  - Optional log files: Configurable via logging.file setting

**Caching:**
- None (stateless proxy design)
- In-memory rate limit tracking per key per provider
- In-memory circuit breaker state

**API Key Storage:**
- Configuration file (`config.yaml`) - keys referenced via environment variables
- Environment variables: Primary mechanism for storing sensitive credentials
- No built-in secure vault integration (planned feature)

## Authentication & Identity

**Auth Provider:**
- Custom multi-provider authentication system

**Implementation Details:**

| Provider | Method | Credential Storage | Refresh |
|----------|--------|-------------------|---------|
| Anthropic | x-api-key header | Environment variable | Static |
| Z.AI | ANTHROPIC_AUTH_TOKEN header | Environment variable | Static |
| Ollama | None | N/A | N/A |
| Bedrock | AWS SigV4 or Bearer Token | AWS credentials or environment var | SigV4 per-request or static token |
| Azure | x-api-key or Entra ID OAuth | Environment variables | Static or cached OAuth token |
| Vertex AI | Google OAuth | Service account file (GOOGLE_APPLICATION_CREDENTIALS) | Refreshed per request |

**Environment Variables Used:**
- `ANTHROPIC_API_KEY` - Anthropic API key
- `ZAI_API_KEY` - Z.AI API key
- `AWS_BEARER_TOKEN_BEDROCK` - AWS Bedrock bearer token (alternative to SigV4)
- `AZURE_API_KEY` - Azure API key
- `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET` - Azure Entra ID credentials
- `GOOGLE_APPLICATION_CREDENTIALS` - Path to Google service account JSON
- `GOOGLE_CLOUD_PROJECT` - Google Cloud project ID
- Standard AWS environment variables for SigV4: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`

## Monitoring & Observability

**Error Tracking:**
- None (custom logging system planned)
- Circuit breaker captures errors by type: rate limits (429), timeouts, server errors (5xx)

**Logs:**
- Structured logging via `log/slog` or similar
- Configurable levels: debug, info, warn, error
- Formats: JSON or text
- Optional file output via logging.file configuration
- Request/response logging (detail level TBD)

**Metrics:**
- Prometheus metrics export (planned)
- Endpoint: `/metrics` (default)
- Listen address: `127.0.0.1:9100` (default)
- Metrics tracked:
  - Total requests per provider
  - Success/failure counts
  - Token usage (in/out)
  - Latency (avg, P50, P95, P99)
  - Rate limit usage (RPM/TPM per key)
  - In-flight requests
  - Circuit breaker state

## CI/CD & Deployment

**Hosting:**
- Self-hosted daemon (single binary)
- Docker container support (planned, not yet specified)
- Supported platforms: Linux, Windows, macOS

**CI Pipeline:**
- GitHub Actions (configured in `.github/workflows/test.yml`)
- Build and test on push
- Test coverage collection

**Deployment Model:**
- Standalone binary: `cc-relay serve` (daemon mode)
- Local-only by default (127.0.0.1 listening)
- Can be exposed via reverse proxy for remote access

## Environment Configuration

**Required env vars by provider:**

| Provider | Required Variables |
|----------|-------------------|
| Anthropic | `ANTHROPIC_API_KEY` |
| Z.AI | `ZAI_API_KEY` |
| Ollama | None |
| Bedrock | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` OR `AWS_BEARER_TOKEN_BEDROCK` |
| Azure (key auth) | `AZURE_API_KEY` |
| Azure (Entra ID) | `AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET` |
| Vertex AI | `GOOGLE_APPLICATION_CREDENTIALS`, `GOOGLE_CLOUD_PROJECT` |

**Secrets location:**
- Environment variables (recommended)
- Configuration file with environment variable references: `${VAR_NAME}` syntax
- No built-in secrets management (relies on OS-level env var management)

## Webhooks & Callbacks

**Incoming:**
- None (stateless HTTP proxy)

**Outgoing:**
- None directly to external services
- Health checks to backends: Periodic probes to provider health endpoints
- Optional monitoring webhooks via logging integration (future feature)

## Request/Response Flow

**Claude Code → cc-relay → Provider:**

1. Claude Code sends Anthropic Messages API request to `http://localhost:8787/v1/messages`
2. cc-relay receives request with:
   - Headers: `x-api-key`, `anthropic-version`, `content-type: application/json`
   - Body: Standard Anthropic Messages API format
3. Router selects provider + API key based on strategy
4. Provider transformer adapts request for target provider
   - May transform auth headers
   - May transform request body (Bedrock, Vertex AI URL path transformations)
   - May apply model mapping
5. Request sent to provider backend
6. Provider sends response
7. Provider transformer adapts response back to Anthropic format
8. SSE events streamed back to Claude Code in correct sequence:
   - `message_start`
   - `content_block_start`
   - `content_block_delta` (multiple)
   - `content_block_stop`
   - `message_delta`
   - `message_stop`

**SSE Header Requirements:**
```
Content-Type: text/event-stream
Cache-Control: no-cache, no-transform
X-Accel-Buffering: no
Connection: keep-alive
```

## Provider-Specific Details

### Bedrock Transformations
- Model specified in URL path, not body
- Request body includes `anthropic_version: "bedrock-2023-05-31"`
- Auth: AWS SigV4 signing per request or bearer token

### Vertex AI Transformations
- Model specified in URL path
- Request body includes `anthropic_version: "vertex-2023-10-16"`
- Auth: Google OAuth bearer token

### Azure Transformations
- Full Anthropic format
- Auth via `x-api-key` header (not `api-key`)
- Model IDs are Azure deployment names

### Ollama Limitations
- No prompt caching support
- No extended thinking enforcement
- No PDF vision support
- Images must be base64 encoded (no URLs)

---

*Integration audit: 2026-01-20*
