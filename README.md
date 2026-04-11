<div align="center">

# 🦅 ZClaw

**Docker-native, headless-first, always-on AI agent platform**

Run **100–300 AI agents on a single server** with near-zero idle overhead.

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/zclaw/zclaw)](https://goreportcard.com/report/github.com/zclaw/zclaw)
[![Status](https://img.shields.io/badge/Status-Alpha-orange.svg)]()

[Quick Start](#-quick-start) · [Architecture](#-architecture) · [Providers](#-providers) · [Auth & Security](#-auth--security) · [API Reference](#-api-reference) · [CLI Reference](#-cli-reference) · [Roadmap](#-roadmap)

</div>

---

> **300 agents. One server. Near-zero idle CPU.**
> Agents are database rows, not containers. Execution contexts are only created when tasks actually run.

| | ZClaw | Desktop-first agents |
|---|---|---|
| **Agent density** | 300 on one server ✅ | OOM likely ❌ |
| **Deploy model** | Headless-first ✅ | Optional ⚠️ |
| **Install** | `docker compose up -d` ✅ | Manual setup ⚠️ |
| **Idle overhead** | ~0% per agent | 1–5% per agent |
| **Browser** | Pooled headless sidecar | Per-agent browser |
| **Providers** | 20 adapters | Varies |
| **Auth** | RBAC + API keys + sessions | Rarely |

---

## 🚀 Quick Start

### Prerequisites

- Linux server or VM
- Docker Engine 20.10+
- Docker Compose v2
- At least one LLM API key

### 1. Clone and install

```bash
git clone https://github.com/FeverDream-dev/Zclaw.git
cd Zclaw
./scripts/install.sh
```

### 2. Configure providers

```bash
cp .env.example .env
# Edit .env — add your API keys:
# OPENAI_API_KEY=sk-...
# ANTHROPIC_API_KEY=sk-ant-...
```

### 3. Start the stack

```bash
docker compose -f docker/compose.yaml up -d
```

### 4. Create your first agent

```bash
docker compose -f docker/compose.yaml exec control-plane \
  dockclawctl agent create "my-agent" --provider openai --model gpt-4o-mini
```

### 5. Verify it works

```bash
# Health check
curl http://localhost:8081/health
# {"status":"healthy","version":"0.1.0",...}

# System stats
dockclawctl stats
# {
#   "agents_total": 1,
#   "agents_active": 0,
#   "queue_depth": 0,
#   "active_workers": 0,
#   "provider_requests": 0,
#   "tokens_used": 0
# }
```

---

## 🏗 Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                     ZClaw Control Plane                      │
│                                                              │
│  ┌───────────┐ ┌───────────┐ ┌────────────┐ ┌────────────┐ │
│  │  Agent     │ │ Scheduler │ │  Provider  │ │   Auth     │ │
│  │ Registry   │ │  Engine   │ │  Gateway   │ │  Service   │ │
│  └───────────┘ └───────────┘ └────────────┘ └────────────┘ │
│  ┌───────────┐ ┌───────────┐ ┌────────────┐ ┌────────────┐ │
│  │  Task     │ │ Browser   │ │   Config   │ │  Health &  │ │
│  │  Queue    │ │ Sessions  │ │  Package   │ │ Telemetry  │ │
│  └───────────┘ └───────────┘ └────────────┘ └────────────┘ │
│                                                              │
│  ── Middleware: CORS · Recovery · Logging · RateLimit · Auth ─│
│  ── API: REST (8080) · Dashboard · Health (8081) ─────────── │
│  ── Storage: SQLite WAL · V1–V12 Migrations ──────────────── │
└──────────────────────┬───────────────────────────────────────┘
                       │ Docker API
          ┌────────────┼────────────┐
          ▼            ▼            ▼
    ┌───────────┐ ┌──────────┐ ┌──────────┐
    │   Tool    │ │ Browser  │ │   More   │
    │  Worker   │ │  Worker  │ │ Workers  │
    │  (shell)  │ │(Playwright)│ (future) │
    └───────────┘ └──────────┘ └──────────┘
```

### Request flow

```
Request → CORS → Recovery → Logging → RateLimit → Auth Middleware
                                                       │
                                              ┌────────┴────────┐
                                              │ Bearer token?   │
                                              │ ApiKey header?  │
                                              │ Admin bypass?   │
                                              └────────┬────────┘
                                                       │
                                              AuthContext (role, tenant)
                                                       │
                                              Route Handler
                                                       │
                                              Provider Gateway → LLM API
                                                       │
                                              Response (JSON)
```

### Design principles

| # | Principle | Why it matters |
|---|---|---|
| 1 | **Agents are rows, not daemons** | 300 agents = 300 DB rows, not 300 containers or goroutines |
| 2 | **Workers are pooled** | Execution contexts created on demand, capped, and cleaned up |
| 3 | **Headless-first** | Every feature works without a monitor or browser |
| 4 | **Docker-native** | `docker compose up -d` is the full install path |
| 5 | **Provider-agile** | All LLM differences behind adapter boundaries — one interface, 20 implementations |
| 6 | **Event-driven heartbeat** | Wake agents on events, not polling — zero CPU when idle |
| 7 | **Auth is optional** | Disable auth for single-user; enable RBAC for multi-tenant production |

---

## 🤖 Providers

### Major Cloud

| Provider | Auth | Env Variable | Tools | Stream | Vision | Key Models |
|---|---|---|---|---|---|---|
| **OpenAI** | API Key | `OPENAI_API_KEY` | ✅ | ✅ | ✅ | GPT-4o, GPT-4o-mini, o1, o3 |
| **Anthropic** | API Key | `ANTHROPIC_API_KEY` | ✅ | ✅ | ✅ | Claude Sonnet 4, Claude Opus 4 |
| **Google Gemini** | API Key | `GEMINI_API_KEY` | ✅ | ✅ | ✅ | Gemini Pro, Gemini Flash |

### Open Source Gateways

| Provider | Auth | Env Variable | Tools | Stream | Vision | Key Models |
|---|---|---|---|---|---|---|
| **OpenRouter** | API Key | `OPENROUTER_API_KEY` | ✅ | ✅ | ✅ | Gateway to 100+ models |
| **Together AI** | API Key | `TOGETHER_API_KEY` | ✅ | ✅ | ❌ | Llama 3.3/3.1 70B/405B, Qwen, DeepSeek-R1, Mixtral |
| **Fireworks** | API Key | `FIREWORKS_API_KEY` | ✅ | ✅ | ✅ | Fast open-source inference |
| **LiteLLM** | API Key | `LITELLM_API_KEY` + `LITELLM_BASE_URL` | ✅ | ✅ | ✅ | Generic OpenAI-compatible proxy |

### Cloud Platforms

| Provider | Auth | Env Variable | Tools | Stream | Vision | Key Models |
|---|---|---|---|---|---|---|
| **AWS Bedrock** | Access Key | `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` | ✅ | ✅ | ✅ | Multi-model gateway |
| **Google Vertex AI** | OAuth Token | `VERTEX_AI_ACCESS_TOKEN` + `VERTEX_AI_PROJECT_ID` | ✅ | ✅ | ✅ | Gemini, Claude, Llama via Google Cloud |
| **Cloudflare AI** | API Key | `CF_ACCOUNT_ID` + `CF_API_KEY` | ✅ | ✅ | ✅ | Workers AI Gateway |

### Chinese AI

| Provider | Auth | Env Variable | Tools | Stream | Vision | Key Models |
|---|---|---|---|---|---|---|
| **Z.AI (Zhipu)** | API Key | `ZAI_API_KEY` | ✅ | ✅ | ✅ | GLM-5.1, GLM-4.7, GLM-4V + Coding Plan |
| **DeepSeek** | API Key | `DEEPSEEK_API_KEY` | ✅ | ✅ | ❌ | DeepSeek-V3, R1 (reasoning), Coder-V2 |
| **Alibaba Qwen** | API Key | `QWEN_API_KEY` | ✅ | ✅ | ✅ | Qwen 2.5 series |
| **MiniMax** | API Key | `MINIMAX_API_KEY` | ✅ | ✅ | ❌ | MiniMax-Text-01 (1M ctx), M1 (reasoning), abab6.5 |
| **Moonshot AI** | API Key | `MOONSHOT_API_KEY` | ❌ | ✅ | ❌ | Kimi models, 8K–128K context |

### Self-Hosted

| Provider | Auth | Env Variable | Tools | Stream | Vision | Key Models |
|---|---|---|---|---|---|---|
| **Ollama** | None | `OLLAMA_BASE_URL` | ✅ | ✅ | ❌ | Any local model, free |
| **Ollama Cloud** | API Key | `OLLAMA_CLOUD_API_KEY` | ✅ | ✅ | ❌ | Hosted models: GPT-OSS, DeepSeek, Qwen |

### Fast Inference

| Provider | Auth | Env Variable | Tools | Stream | Vision | Key Models |
|---|---|---|---|---|---|---|
| **Groq** | API Key | `GROQ_API_KEY` | ✅ | ✅ | ❌ | Llama, Mixtral — sub-100ms TTFT |
| **Mistral** | API Key | `MISTRAL_API_KEY` | ✅ | ✅ | ✅ | Mistral Large, Medium, Small |
| **xAI** | API Key | `XAI_API_KEY` | ✅ | ✅ | ✅ | Grok models |

Adding a provider is a single Go file implementing the [`Provider`](internal/providers/provider.go) interface:

```go
type Provider interface {
    ID() ProviderID
    Capabilities() []CapabilityFlag
    Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
    GenerateStream(ctx context.Context, req GenerateRequest) (<-chan StreamChunk, error)
    ValidateConfig(config ProviderConfig) error
    NormalizeModel(model string) string
    Close() error
}
```

---

## 🔧 Built-in Tools (20)

<details>
<summary><strong>Web & HTTP</strong></summary>

| Tool | ID | Description | Key Parameters | Timeout |
|---|---|---|---|---|
| **WebFetch** | `web_fetch` | Fetch URL content via HTTP GET | `url` (required), `headers` | 15s |
| **HTTPRequest** | `http_request` | Make any HTTP request | `method` (required), `url` (required), `headers`, `body` | 20s |

</details>

<details>
<summary><strong>File Operations</strong></summary>

| Tool | ID | Description | Key Parameters | Timeout |
|---|---|---|---|---|
| **FileRead** | `file_read` | Read a file from workspace | `path` (required) | 10s |
| **FileWrite** | `file_write` | Write content to workspace file | `path` (required), `content` (required) | 10s |
| **ListFiles** | `list_files` | List directory contents | `path` | 5s |

</details>

<details>
<summary><strong>Shell & Code Execution</strong></summary>

| Tool | ID | Description | Key Parameters | Timeout |
|---|---|---|---|---|
| **ShellExec** | `shell_exec` | Execute shell commands | `command` (required), `workdir` | 30s |
| **PythonExec** | `python_exec` | Run Python code in subprocess | `code` or `script` | 60s |
| **JavaScriptExec** | `js_exec` | Run JavaScript with Node.js | `code` or `script` | 60s |
| **GoEval** | `go_eval` | Evaluate a simple numeric expression | `expr` (required) | 15s |

</details>

<details>
<summary><strong>Data Processing</strong></summary>

| Tool | ID | Description | Key Parameters | Timeout |
|---|---|---|---|---|
| **CSVRead** | `csv_read` | Parse CSV into JSON | `path` (required) | 15s |
| **JSONParse** | `json_parse` | Parse JSON and query by dot-path | `json` (required), `path` | 5s |
| **TextSearch** | `text_search` | Regex search over text | `text` (required), `pattern` (required) | 5s |
| **TextReplace** | `text_replace` | Find and replace text | `text` (required), `old` (required), `new` (required) | 5s |
| **Base64Encode** | `base64_encode` | Encode string to Base64 | `input` (required) | 5s |
| **Base64Decode** | `base64_decode` | Decode Base64 string | `input` (required) | 5s |
| **Hash** | `hash` | Compute SHA256 + MD5 | `input` (required) | 5s |

</details>

<details>
<summary><strong>System & Utility</strong></summary>

| Tool | ID | Description | Key Parameters | Timeout |
|---|---|---|---|---|
| **DiskUsage** | `disk_usage` | Compute disk usage for a path | `path` | 30s |
| **Env** | `env` | Read environment variables | `allow` (filter list) | 5s |
| **Timestamp** | `timestamp` | Current timestamp in Unix/ISO/RFC3339 | — | 5s |
| **Wait** | `wait` | Sleep for N seconds | `duration` (required) | 60s |

</details>

All tools execute with per-tool timeouts and return structured `ToolResult`:

```json
{
  "tool_id": "web_fetch",
  "success": true,
  "output": "<html>...",
  "error": "",
  "artifacts": [],
  "duration": 234000000
}
```

---

## 🔐 Auth & Security

### Auth architecture

```
┌──────────────────────────────────────────────────────────┐
│                    Request Pipeline                       │
│                                                          │
│  Request ──► CORS ──► Recovery ──► Logging ──► RateLimit │
│                                              │           │
│                                     AuthMiddleware       │
│                                     ┌────────┴────────┐ │
│                                     │ Auth disabled?   │ │
│                                     │ → admin context  │ │
│                                     │                  │ │
│                                     │ Bearer <prefix>? │ │
│                                     │ → API key auth   │ │
│                                     │                  │ │
│                                     │ Bearer <token>?  │ │
│                                     │ → Session auth   │ │
│                                     │                  │ │
│                                     │ ApiKey <key>?    │ │
│                                     │ → API key auth   │ │
│                                     │                  │ │
│                                     │ Admin API key?   │ │
│                                     │ → admin bypass   │ │
│                                     └────────┬────────┘ │
│                                              │           │
│                                    RequirePermission(perm) │
│                                              │           │
│                                         Route Handler     │
└──────────────────────────────────────────────────────────┘
```

### Role hierarchy

| Role | Permissions |
|---|---|
| **admin** | All — agents, tasks, tools, dashboard, users, tenants, API keys |
| **operator** | Agents (CRUD), tasks, tools, dashboard |
| **viewer** | Read agents, read tasks, dashboard |
| **agent** | Enqueue tasks, execute tools |
| **anonymous** | None |

### Permission matrix

| Permission | admin | operator | viewer | agent | anonymous |
|---|:---:|:---:|:---:|:---:|:---:|
| `agent:create` | ✅ | ✅ | — | — | — |
| `agent:read` | ✅ | ✅ | ✅ | — | — |
| `agent:update` | ✅ | ✅ | — | — | — |
| `agent:delete` | ✅ | ✅ | — | — | — |
| `task:enqueue` | ✅ | ✅ | — | ✅ | — |
| `task:read` | ✅ | ✅ | ✅ | — | — |
| `tool:execute` | ✅ | ✅ | — | ✅ | — |
| `dashboard:read` | ✅ | ✅ | ✅ | — | — |
| `user:manage` | ✅ | — | — | — | — |
| `tenant:manage` | ✅ | — | — | — | — |
| `apikey:manage` | ✅ | — | — | — | — |

### API authentication methods

| Method | Header | Example |
|---|---|---|
| Bearer token (session) | `Authorization: Bearer <token>` | `Authorization: Bearer eyJ...` |
| Bearer token (API key) | `Authorization: Bearer zclaw_...` | `Authorization: Bearer zclaw_a1b2c3d4...` |
| ApiKey header | `Authorization: ApiKey <key>` | `Authorization: ApiKey zclaw_...` |
| Admin bypass | `Authorization: Bearer <admin_key>` | Matches `ZCLAW_ADMIN_API_KEY` env var |

### Session tokens

Session tokens use HMAC-SHA256 signed JSON payloads (no external JWT library):

```
base64url({"sub":"user_id","tid":"tenant_id","role":"admin","exp":...,"iat":...}).<hmac_signature>
```

- Tokens are verified against `ZCLAW_JWT_SECRET`
- Tokens expire based on `ZCLAW_SESSION_TTL` (default: 24h)
- No external dependencies — pure Go `crypto/hmac`

### Auth configuration

| Variable | Default | Description |
|---|---|---|
| `ZCLAW_AUTH_ENABLED` | `false` | Enable/disable authentication |
| `ZCLAW_JWT_SECRET` | — | HMAC secret for session tokens (required if auth enabled) |
| `ZCLAW_ADMIN_API_KEY` | — | Admin bypass key — full access without DB lookup |
| `ZCLAW_SESSION_TTL` | `24h` | Session token lifetime |
| `ZCLAW_API_KEY_PREFIX` | `zclaw_` | Prefix for auto-generated API keys |

When auth is disabled, all requests run as admin with no authentication required.

### Security checklist

- Rootless containers where practical
- Per-agent filesystem boundaries (read-only or read-write policies)
- Browser and tool workers isolated in separate containers
- Docker socket never exposed to agent code
- Per-agent CPU, memory, and timeout limits
- Network policy options: `full`, `restricted`, `none`
- Full audit trail for tool and browser actions
- API keys stored as SHA-256 hashes (plaintext never persisted)
- Rate limiting with configurable RPS and burst

---

## 📡 API Reference

| Method | Path | Description | Auth |
|---|---|---|---|
| `GET` | `/health` | Health check (port 8081) | Public |
| `GET` | `/api/v1/stats` | System statistics | Required |
| `GET` | `/api/v1/providers` | List registered providers | Required |

<details>
<summary><strong>Agent endpoints</strong></summary>

| Method | Path | Description | Auth |
|---|---|---|---|
| `GET` | `/api/v1/agents` | List agents (paginated, filterable) | `agent:read` |
| `POST` | `/api/v1/agents` | Create agent | `agent:create` |
| `GET` | `/api/v1/agents/{id}` | Get agent details | `agent:read` |
| `PATCH` | `/api/v1/agents/{id}` | Update agent | `agent:update` |
| `DELETE` | `/api/v1/agents/{id}` | Delete agent | `agent:delete` |

</details>

<details>
<summary><strong>Task, Tool, Sub-Agent, Template, Connection endpoints</strong></summary>

| Method | Path | Description | Auth |
|---|---|---|---|
| `POST` | `/api/v1/tasks` | Enqueue a task for an agent | `task:enqueue` |
| `GET` | `/api/v1/tools` | List all registered tools | `tool:execute` |
| `GET` | `/api/v1/tools/{id}` | Get tool specification | `tool:execute` |
| `POST` | `/api/v1/tools/{id}/execute` | Execute a tool with JSON params | `tool:execute` |
| `POST` | `/api/v1/subagents` | Spawn a sub-agent | `agent:create` |
| `GET` | `/api/v1/subagents/{id}` | Get sub-agent details | `agent:read` |
| `GET` | `/api/v1/subagents/parent/{parentId}` | List sub-agents for parent | `agent:read` |
| `POST` | `/api/v1/subagents/{id}/cancel` | Cancel a sub-agent | `agent:update` |
| `GET` | `/api/v1/templates` | List agent templates | `agent:read` |
| `POST` | `/api/v1/templates/{name}/instantiate` | Create agent from template | `agent:create` |
| `GET` | `/api/v1/connections` | List connection statuses | Required |

</details>

<details>
<summary><strong>Dashboard endpoints</strong></summary>

| Method | Path | Description | Auth |
|---|---|---|---|
| `GET` | `/api/v1/dashboard` | Fleet overview | `dashboard:read` |
| `GET` | `/api/v1/dashboard/stats` | Dashboard statistics | `dashboard:read` |
| `GET` | `/api/v1/dashboard/agents/` | Agent detail/actions | `dashboard:read` |
| `GET` | `/api/v1/dashboard/providers/health` | Provider health status | `dashboard:read` |
| `GET` | `/api/v1/dashboard/workers` | Worker pool status | `dashboard:read` |
| `GET` | `/api/v1/dashboard/queue` | Task queue status | `dashboard:read` |
| `GET` | `/api/v1/dashboard/activity` | Recent activity log | `dashboard:read` |
| `GET` | `/api/v1/dashboard/errors` | Recent errors | `dashboard:read` |
| `GET` | `/api/v1/dashboard/export` | Export all agents as JSON | `dashboard:read` |

</details>

### Example requests

```bash
# Create an agent
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "name": "research-bot",
    "provider": {
      "provider_id": "openai",
      "model": "gpt-4o-mini"
    },
    "schedule": {
      "cron": "0 9 * * 1-5",
      "enabled": true
    }
  }'

# Enqueue a task
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"<id>","input":"Analyze the latest sales data"}'

# Execute a tool
curl -X POST http://localhost:8080/api/v1/tools/web_fetch/execute \
  -H "Content-Type: application/json" \
  -d '{"url":"https://api.example.com/status"}'
```

---

## 🖥 CLI Reference

`dockclawctl` is a Cobra-based CLI that talks to the control plane API.

### Configuration

```bash
# Override API URL (default: http://localhost:8080)
export ZCLAW_API_URL=http://my-server:8080
dockclawctl --api-url http://my-server:8080 ...
```

### Agent management

```bash
dockclawctl agent list                                    # List all agents
dockclawctl agent list --state active --limit 100         # Filtered list
dockclawctl agent create "bot" --provider openai --model gpt-4o-mini
dockclawctl agent create "scheduled" --provider anthropic --model claude-sonnet-4-20250514 --schedule "0 */6 * * *"
dockclawctl agent get <id>                                # Show agent details (JSON)
dockclawctl agent pause <id>                              # Pause an agent
dockclawctl agent resume <id>                             # Resume a paused agent
dockclawctl agent disable <id>                            # Disable an agent
dockclawctl agent delete <id>                             # Delete an agent
```

### Task management

```bash
dockclawctl task enqueue <agent-id> "analyze this data"  # Send task to agent
```

### Sub-agents

```bash
dockclawctl subagent spawn <parent-id> <name> <task>     # Spawn a child agent
dockclawctl subagent list <parent-id>                     # List sub-agents
dockclawctl subagent cancel <id>                          # Cancel a sub-agent
```

### Templates

```bash
dockclawctl template list                                 # List agent templates
dockclawctl template instantiate <name> <parent-id>       # Create agent from template
```

### Tools

```bash
dockclawctl tool list                                     # List all registered tools
dockclawctl tool get <id>                                 # Show tool specification
dockclawctl tool execute <id> '{"url":"https://..."}'    # Execute a tool
```

### Connections & Dashboard

```bash
dockclawctl connection list                               # Show connection statuses
dockclawctl dashboard overview                            # Fleet overview
dockclawctl dashboard stats                               # Dashboard stats
dockclawctl dashboard errors                              # Recent errors
dockclawctl dashboard activity                            # Recent activity
dockclawctl dashboard export                              # Export all agents as JSON
```

### System

```bash
dockclawctl stats                                         # Show system stats
dockclawctl provider list                                 # List registered providers
dockclawctl doctor                                        # Check API connectivity
```

---

## 🔌 Connections

| Type | Description |
|---|---|
| **MCP Server** | JSON-RPC server for MCP tool integration (`ZCLAW_MCP_ENABLED=true`) |
| **WebSocket Bridge** | Polling bridge for real-time clients |
| **Webhooks** | Sender/receiver with HMAC signing |
| **File Watcher** | Polling filesystem watcher — triggers agent tasks on file changes |

Managed by the unified `ConnectionManager`, exposed via `GET /api/v1/connections`.

---

## 📦 Project Structure

```
Zclaw/
├── cmd/
│   ├── dockclawd/            # Control plane daemon (main entry)
│   └── dockclawctl/          # CLI admin tool (Cobra-based)
├── internal/
│   ├── agents/               # Agent registry, lifecycle, sub-agents, templates
│   ├── api/                  # REST routes, middleware, portal dashboard, auth handlers
│   ├── auth/                 # RBAC roles, permissions, session tokens, API key management
│   ├── config/               # Environment variable loading and validation
│   ├── providers/            # Provider interface & registry
│   │   └── adapters/         # 20 adapters: OpenAI, Anthropic, OpenRouter, Ollama,
│   │                        #   Ollama Cloud, LiteLLM, Groq, Gemini, Mistral, xAI,
│   │                        #   Bedrock, Qwen, Fireworks, Cloudflare, Z.AI, DeepSeek,
│   │                        #   Together, MiniMax, Moonshot, Vertex AI
│   ├── tools/                # 20 built-in tools (web, file, code, data, system)
│   ├── connections/          # MCP, WebSocket, webhooks, file watchers
│   ├── storage/              # SQLite WAL + migrations V1–V12, auth repos
│   ├── scheduler/            # Event-driven task scheduler
│   ├── runtime/              # Docker worker pool management
│   ├── browser/              # Browser session pooling (Playwright)
│   ├── memory/               # Conversations & artifacts
│   ├── telemetry/            # Health checks, metrics, audit logging
│   └── channels/             # Message routing (future)
├── browser-worker/           # Node.js / Playwright sidecar
├── docker/
│   ├── compose.yaml          # Production stack (3 services)
│   ├── compose.dev.yaml      # Dev overrides (hot reload)
│   ├── env.example           # Docker environment template
│   └── images/               # Dockerfiles (control-plane, browser-worker, tool-worker)
├── scripts/                  # install, update, doctor, backup, scale-agents
├── docs/adr/                 # Architecture Decision Records
└── examples/                 # Agent templates, policies, provider configs
```

**15 Go packages** · ~12K lines of Go code · 20 provider adapters · 20 tools · 12 database migrations

---

## ⚙️ Configuration

### Complete environment variable reference

#### Core

| Variable | Default | Description |
|---|---|---|
| `ZCLAW_DATA_DIR` | `./data` | Data directory for SQLite and workspaces |
| `ZCLAW_DB_PATH` | `./data/zclaw.db` | SQLite database file path |
| `ZCLAW_HTTP_PORT` | `8080` | API server port |
| `ZCLAW_HEALTH_PORT` | `8081` | Health check port |
| `ZCLAW_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `ZCLAW_DOCKER_SOCKET` | `/var/run/docker.sock` | Docker socket path |

#### Workers & Scheduling

| Variable | Default | Description |
|---|---|---|
| `ZCLAW_WORKER_POOL_SIZE` | `10` | Maximum concurrent task workers |
| `ZCLAW_BROWSER_POOL_SIZE` | `5` | Maximum concurrent browser sessions |
| `ZCLAW_BROWSER_WORKER_URL` | `ws://browser-worker:9222` | Playwright WebSocket URL |
| `ZCLAW_HEARTBEAT_JITTER_SECONDS` | `30` | Schedule jitter to avoid thundering herd |
| `ZCLAW_DEFAULT_MODEL_PROVIDER` | `openai` | Default provider for new agents |
| `ZCLAW_DEFAULT_MODEL` | `gpt-4o-mini` | Default model for new agents |

#### Auth & Rate Limiting

| Variable | Default | Description |
|---|---|---|
| `ZCLAW_AUTH_ENABLED` | `false` | Enable RBAC authentication |
| `ZCLAW_JWT_SECRET` | — | HMAC secret for session tokens |
| `ZCLAW_ADMIN_API_KEY` | — | Admin bypass key (full access) |
| `ZCLAW_SESSION_TTL` | `24h` | Session token lifetime |
| `ZCLAW_API_KEY_PREFIX` | `zclaw_` | Prefix for generated API keys |
| `ZCLAW_RATE_LIMIT_RPS` | `100` | Requests per second (token bucket) |
| `ZCLAW_RATE_LIMIT_BURST` | `200` | Burst capacity for rate limiter |

#### Provider API Keys

| Variable | Provider |
|---|---|
| `OPENAI_API_KEY` | OpenAI |
| `ANTHROPIC_API_KEY` | Anthropic |
| `OPENROUTER_API_KEY` | OpenRouter |
| `GEMINI_API_KEY` | Google Gemini |
| `MISTRAL_API_KEY` | Mistral |
| `XAI_API_KEY` | xAI (Grok) |
| `GROQ_API_KEY` | Groq |
| `QWEN_API_KEY` | Alibaba Qwen |
| `FIREWORKS_API_KEY` | Fireworks |
| `ZAI_API_KEY` | Z.AI (Zhipu) |
| `DEEPSEEK_API_KEY` | DeepSeek |
| `TOGETHER_API_KEY` | Together AI |
| `MINIMAX_API_KEY` | MiniMax |
| `MOONSHOT_API_KEY` | Moonshot AI |
| `OLLAMA_CLOUD_API_KEY` | Ollama Cloud |
| `OLLAMA_BASE_URL` | Ollama (local) |
| `LITELLM_API_KEY` + `LITELLM_BASE_URL` | LiteLLM proxy |
| `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` | AWS Bedrock |
| `VERTEX_AI_ACCESS_TOKEN` + `VERTEX_AI_PROJECT_ID` | Google Vertex AI |
| `CF_ACCOUNT_ID` + `CF_API_KEY` + `CF_GATEWAY_ID` | Cloudflare AI |

---

## 📋 Operator Guide

```bash
./scripts/install.sh              # Full stack setup on a fresh server
./scripts/update.sh               # Safe update with backup + migration
./scripts/backup.sh               # SQLite + config + workspace archival
./scripts/doctor.sh               # 10-point health check
./scripts/scale-agents.sh --count 100  # Bulk create test agents
```

Built-in backup command: `dockclawd backup-db`

Docker resource limits (production compose): Control plane 512MB/0.5 CPU · Browser worker 1GB/1 CPU · Tool worker 256MB/0.25 CPU

---

## 🧪 Development

### Dev mode with hot reload

```bash
docker compose -f docker/compose.yaml -f docker/compose.dev.yaml up
go build ./cmd/dockclawd      # Build control plane
go build ./cmd/dockclawctl    # Build CLI
go test ./...                 # Run all tests
```

### Adding a new provider adapter

1. Create `internal/providers/adapters/myprovider.go`
2. Implement the `Provider` interface (ID, Capabilities, Generate, GenerateStream, ValidateConfig, NormalizeModel, Close)
3. Register in `cmd/dockclawd/main.go` → `registerProviders()`
4. Add env variable to `.env.example`

<details>
<summary>Example provider adapter implementation</summary>

```go
package adapters

import (
    "context"
    "github.com/zclaw/zclaw/internal/providers"
)

type MyProviderAdapter struct {
    apiKey  string
    baseURL string
}

func NewMyProviderAdapter(apiKey, baseURL string) *MyProviderAdapter {
    return &MyProviderAdapter{apiKey: apiKey, baseURL: baseURL}
}

func (a *MyProviderAdapter) ID() providers.ProviderID {
    return providers.ProviderID("myprovider")
}

func (a *MyProviderAdapter) Capabilities() []providers.CapabilityFlag {
    return []providers.CapabilityFlag{
        providers.CapTools, providers.CapStreaming, providers.CapVision,
    }
}

func (a *MyProviderAdapter) HasCapability(cap providers.CapabilityFlag) bool {
    for _, c := range a.Capabilities() {
        if c == cap { return true }
    }
    return false
}

func (a *MyProviderAdapter) Generate(ctx context.Context, req providers.GenerateRequest) (*providers.GenerateResponse, error) {
    // HTTP call to provider API → map response to GenerateResponse
    return &providers.GenerateResponse{}, nil
}

func (a *MyProviderAdapter) GenerateStream(ctx context.Context, req providers.GenerateRequest) (<-chan providers.StreamChunk, error) {
    ch := make(chan providers.StreamChunk)
    // Stream from provider, send chunks on channel
    return ch, nil
}

func (a *MyProviderAdapter) ValidateConfig(config providers.ProviderConfig) error { return nil }
func (a *MyProviderAdapter) NormalizeModel(model string) string { return model }
func (a *MyProviderAdapter) Close() error { return nil }
```

</details>

### Adding a new tool

1. Create the tool in the appropriate file under `internal/tools/`
2. Implement the `ToolExecutor` interface (`Execute` + `Spec`)
3. Register in `registerBuiltinTools()` in `cmd/dockclawd/main.go`

### Database migrations

Sequential SQL constants in `internal/storage/db.go`. To add: define `migrationV13`, append to the `migrations` slice. Idempotent — tracked in `schema_migrations`.

| Version | Tables |
|---|---|
| V1 | `agents` |
| V2 | `agent_schedules` |
| V3 | `agent_policies` |
| V4 | `tasks` |
| V5 | `provider_configs` |
| V6 | `conversations`, `messages` |
| V7 | `artifacts`, `audit_log` |
| V8 | `subagents`, `agent_templates` |
| V9 | `tool_executions`, `connections` |
| V10 | `tenants`, `projects` |
| V11 | `users`, `api_keys` |
| V12 | `chat_sessions`, `widget_profiles` |

---

## 🗺 Roadmap

### Phase 0–1 ✅ *Foundation*

- [x] Architecture Decision Records
- [x] Go control plane with SQLite storage
- [x] Docker Compose stack + operator scripts
- [x] Agent CRUD via API and CLI
- [x] 13 provider adapters
- [x] Playwright browser worker
- [x] Scheduler with event-driven wakeup
- [x] Health checks and telemetry

### Phase 2–3 ✅ *Tools & Connectivity*

- [x] Agent templates and policy presets
- [x] Sub-agent spawning system
- [x] 20 built-in tools (web, file, code, data, system)
- [x] MCP / WebSocket / webhook / file watcher connections
- [x] Fleet management dashboard API

### Phase 4 ✅ *Auth & Tenancy*

- [x] RBAC with 5 roles (admin, operator, viewer, agent, anonymous)
- [x] 11 permissions with `RequirePermission` middleware
- [x] API key management (SHA-256 hashed, prefix-based)
- [x] HMAC-SHA256 session tokens
- [x] Multi-tenant data model (tenants, users, projects)
- [x] Database migrations V10–V12
- [x] Config package with env var validation
- [x] Middleware chain (CORS, Recovery, Logging, RateLimit, Auth)

### Phase 5 🔄 *API Completeness*

- [ ] Per-agent workspace management
- [ ] Cron-based schedules with jitter
- [ ] Active-hours windows
- [ ] Browser pooling with strict concurrency
- [ ] Conversation summarization
- [ ] Artifact retention and cleanup

### Phase 6+ *Scale & UI*

- [ ] Frontend dashboard UI
- [ ] Benchmark harness (10/50/100/300 agents)
- [ ] Density tuning and idle measurement
- [ ] Multi-node support (optional Postgres backend)

---

## 📊 Performance

| Metric | Target | Notes |
|---|---|---|
| **Agent density** | 300 agents/server | Agents are DB rows, not processes |
| **Idle CPU** | ~0% | No goroutines or containers per idle agent |
| **Memory footprint** | < 512MB (control plane) | Docker compose limit: 512MB |
| **Worker pool** | 10 concurrent (configurable) | `ZCLAW_WORKER_POOL_SIZE` |
| **Browser pool** | 5 concurrent (configurable) | `ZCLAW_BROWSER_POOL_SIZE` |
| **Rate limit** | 100 RPS, burst 200 | Configurable via env vars |

Benchmarks planned for Phase 6 (10/50/100/300 agent load tests).

---

## ❓ FAQ

**How is this different from AutoGPT / CrewAI / LangGraph?**

Those are agent *frameworks* — Python libraries you build on top of. ZClaw is an agent *platform* — a standalone service you deploy once and manage via API/CLI. No Python required. No per-agent containers. 300 agents share one process.

**Why SQLite instead of Postgres?**

SQLite with WAL mode handles the write patterns of a single-node agent platform: bursty writes (task completions) with many concurrent reads (agent listing, dashboard queries). It eliminates an entire dependency. Postgres support is on the roadmap for multi-node deployments.

**Can I run this on a $5/month VPS?**

Yes. The control plane runs in 512MB. With auth disabled and no browser worker, you can run 50+ agents on a 1GB VPS. Browser automation requires more memory (2GB+ for the Playwright sidecar).

**How do agents share state?**

Agents can share state through the filesystem (shared `/data` volume) or through the API — one agent's task output can be another agent's input. Sub-agents provide a parent-child hierarchy for coordination.

**What happens when a provider goes down?**

Each agent can have a `fallback_provider` and `fallback_model` configured. The scheduler retries failed tasks up to `max_attempts` (default: 3). All failures are logged to the audit trail.

**Can I use multiple providers for different agents?**

Yes. Each agent is configured with its own provider and model. You can have 50 agents on OpenAI, 100 on Anthropic, and 150 on Ollama — all on the same server.

**Is there a web UI?**

A frontend dashboard UI is planned for Phase 6. Today, use the CLI (`dockclawctl dashboard overview`) or the dashboard API endpoints.

**Can I integrate ZClaw into my own application?**

Yes. The REST API is the primary interface. Use the API to create agents, enqueue tasks, and read results. The MCP server enables tool integration with other AI systems.

---

## 🤝 Contributing

Contributions are welcome. The codebase follows standard Go conventions:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Write tests for new functionality
4. Ensure `go test ./...` passes
5. Submit a pull request

**Code style:**
- Standard `gofmt` formatting
- Package-level documentation on all packages
- Errors wrapped with `fmt.Errorf("context: %w", err)`
- Structured logging via `log/slog`

See [CONTRIBUTING.md](CONTRIBUTING.md) for full details.

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

## 🙏 Acknowledgments

Architecturally inspired by:
- **OpenClaw** — provider breadth and heartbeat model
- **OpenHands** — Docker sandbox patterns
- **Goose** — MCP extension model
- **GoGogot** — minimal Go core proof
- **AnythingLLM** — Docker-first operator UX

See [docs/adr/005-licensing.md](docs/adr/005-licensing.md) for code reuse policy.

---

<div align="center">

**Built for headless servers. Optimized for density. Never assumes a screen.**

</div>
