<div align="center">

# 🦅 ZClaw

**Docker-native, headless-first, always-on agent platform**

Run **100–300 AI agents on a single server** with near-zero idle overhead.

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Alpha-orange.svg)]()

[Quick Start](#-quick-start) · [Architecture](#-architecture) · [CLI Reference](#-cli-reference) · [Providers](#-providers) · [Roadmap](#-roadmap)

</div>

---

## What is ZClaw?

ZClaw is a self-hosted AI agent platform built for **headless servers**. Unlike desktop-first agent frameworks, ZClaw treats Docker as the default — not an afterthought.

**Key difference:** Agents are database rows, not containers. 300 idle agents consume near-zero CPU because execution contexts are only created when tasks actually run.

### Why ZClaw over other agent platforms?

| Feature | ZClaw | Desktop-first agents |
|---|---|---|
| 300 agents on one server | ✅ Design target | ❌ OOM likely |
| Headless server deploy | ✅ Default mode | ⚠️ Optional |
| Docker-native install | ✅ One command | ⚠️ Manual setup |
| Idle CPU overhead | ~0% | 1-5% per agent |
| Browser automation | Pooled headless sidecar | Per-agent browser |
| Provider breadth | 25+ providers | Varies |

---

## 🚀 Quick Start

```bash
# Clone
git clone https://github.com/FeverDream-dev/Zclaw.git
cd Zclaw

# Install (one script)
./scripts/install.sh

# Create your first agent
docker compose -f docker/compose.yaml exec control-plane \
  dockclawctl agent create "my-agent" --provider openai --model gpt-4o-mini
```

### Prerequisites
- Linux server (or VM)
- Docker Engine 20.10+
- Docker Compose v2
- At least one LLM API key (OpenAI, Anthropic, etc.)

---

## 🏗 Architecture

```
┌─────────────────────────────────────────────┐
│              ZClaw Control Plane             │
│  ┌──────────┐ ┌──────────┐ ┌─────────────┐ │
│  │  Agent    │ │ Scheduler│ │  Provider   │ │
│  │ Registry  │ │  Engine  │ │   Gateway   │ │
│  └──────────┘ └──────────┘ └─────────────┘ │
│  ┌──────────┐ ┌──────────┐ ┌─────────────┐ │
│  │  Task    │ │ Browser  │ │  Health &   │ │
│  │  Queue   │ │ Sessions │ │  Telemetry  │ │
│  └──────────┘ └──────────┘ └─────────────┘ │
│              SQLite WAL · HTTP API           │
└──────────────────┬──────────────────────────┘
                   │ Docker API
        ┌──────────┼──────────┐
        ▼          ▼          ▼
  ┌──────────┐ ┌────────┐ ┌────────┐
  │   Tool   │ │Browser │ │  More  │
  │  Worker  │ │ Worker │ │Workers │
  │ (shell)  │ │(Playwright)│(future)│
  └──────────┘ └────────┘ └────────┘
```

### Design Principles

1. **Agents are rows, not daemons** — 300 agents = 300 DB rows, not 300 containers
2. **Workers are pooled** — execution contexts created on demand, capped, and cleaned up
3. **Headless-first** — every feature works without a monitor
4. **Docker-native** — `docker compose up -d` is the install path
5. **Provider-agile** — all LLM differences behind adapter boundaries
6. **Event-driven heartbeat** — wake agents on events, not polling

---

## 📦 Project Structure

```
Zclaw/
├── cmd/
│   ├── dockclawd/          # Control plane daemon
│   └── dockclawctl/        # CLI admin tool
├── internal/
│   ├── agents/             # Agent registry, lifecycle, sub-agents, templates
│   ├── api/                # Dashboard portal API (fleet overview, stats)
│   ├── providers/          # Provider interface & registry
│   │   └── adapters/       # 13 adapters: OpenAI, Anthropic, OpenRouter, Ollama,
│   │                      #   LiteLLM, Groq, Gemini, Mistral, xAI, Bedrock,
│   │                      #   Qwen, Fireworks, Cloudflare
│   ├── tools/              # 20+ built-in tools (web, file, code, data, system)
│   ├── connections/        # MCP, WebSocket, webhooks, file watchers
│   ├── storage/            # SQLite WAL + migrations V1-V9
│   ├── scheduler/          # Event-driven task scheduler
│   ├── runtime/            # Docker worker pool management
│   ├── browser/            # Browser session pooling
│   ├── memory/             # Conversations & artifacts
│   ├── telemetry/          # Health, metrics, audit
│   └── channels/           # Message routing (future)
├── browser-worker/         # Node/Playwright sidecar
├── docker/
│   ├── compose.yaml        # Production stack
│   ├── compose.dev.yaml    # Dev overrides
│   └── images/             # Dockerfiles (3 images)
├── scripts/                # install, update, doctor, backup, scale
├── docs/adr/               # Architecture Decision Records
└── examples/               # Agent templates, policies, providers
```

---

## 🖥 CLI Reference

```bash
# Agent management
dockclawctl agent list                                    # List all agents
dockclawctl agent create "bot" --provider openai --model gpt-4o-mini
dockclawctl agent get <id>                                # Show agent details
dockclawctl agent pause <id>                              # Pause an agent
dockclawctl agent resume <id>                             # Resume a paused agent
dockclawctl agent delete <id>                             # Delete an agent

# Task management
dockclawctl task enqueue <agent-id> "analyze this data"  # Send task to agent

# Sub-agents
dockclawctl subagent spawn <parent-id> <name> <task>     # Spawn a child agent
dockclawctl subagent list <parent-id>                     # List sub-agents
dockclawctl subagent cancel <id>                          # Cancel a sub-agent

# Templates
dockclawctl template list                                 # List agent templates
dockclawctl template instantiate <name> <parent-id>       # Create agent from template

# Tools
dockclawctl tool list                                     # List 20+ built-in tools
dockclawctl tool get <id>                                 # Show tool details
dockclawctl tool execute <id> '{"url":"https://..."}'    # Execute a tool

# Connections
dockclawctl connection list                               # Show connection statuses

# Dashboard
dockclawctl dashboard overview                            # Fleet overview
dockclawctl dashboard stats                               # Dashboard stats
dockclawctl dashboard errors                              # Recent errors
dockclawctl dashboard activity                            # Recent activity
dockclawctl dashboard export                              # Export all agents

# System
dockclawctl stats                                         # Show system stats
dockclawctl provider list                                 # List registered providers
dockclawctl doctor                                        # Check system health
```

---

## 🤖 Providers

### Built-in adapters (20 providers)
| Provider | Auth | Tools | Streaming | Vision | Notes |
|---|---|---|---|---|---|
| **OpenAI** | API Key | ✅ | ✅ | ✅ | GPT-4o, GPT-4o-mini, o1 |
| **Anthropic** | API Key | ✅ | ✅ | ✅ | Claude Sonnet 4, Claude Opus 4 |
| **OpenRouter** | API Key | ✅ | ✅ | ✅ | Gateway to 100+ models |
| **Ollama** | None | ✅ | ✅ | ❌ | Local models, free |
| **Ollama Cloud** | API Key | ✅ | ✅ | ❌ | Hosted models: GPT-OSS, DeepSeek, Qwen, etc. |
| **LiteLLM** | API Key | ✅ | ✅ | ✅ | Generic OpenAI-compatible |
| **Groq** | API Key | ✅ | ✅ | ❌ | Fast inference |
| **Google Gemini** | API Key | ✅ | ✅ | ✅ | Gemini Pro, Flash |
| **Mistral** | API Key | ✅ | ✅ | ✅ | Mistral Large, Medium, Small |
| **xAI** | API Key | ✅ | ✅ | ✅ | Grok models |
| **AWS Bedrock** | Access Key | ✅ | ✅ | ✅ | Multi-model gateway |
| **Alibaba Qwen** | API Key | ✅ | ✅ | ✅ | Qwen models |
| **Fireworks** | API Key | ✅ | ✅ | ✅ | Fast open-source inference |
| **Cloudflare AI** | API Key | ✅ | ✅ | ✅ | Workers AI Gateway |
| **Z.AI (Zhipu)** | API Key | ✅ | ✅ | ✅ | GLM-5.1, GLM-4.7, GLM-4V, all variants + Coding Plan |
| **DeepSeek** | API Key | ✅ | ✅ | ❌ | DeepSeek-V3, R1 (reasoning), Coder-V2 |
| **Together AI** | API Key | ✅ | ✅ | ❌ | Llama 3.3/3.1 70B/405B, Qwen, DeepSeek-R1, Mixtral |
| **MiniMax** | API Key | ✅ | ✅ | ❌ | MiniMax-Text-01 (1M ctx), M1 (reasoning), abab6.5 |
| **Moonshot AI** | API Key | ❌ | ✅ | ❌ | Kimi models, 8K–128K context |
| **Google Vertex AI** | OAuth Token | ✅ | ✅ | ✅ | Gemini, Claude, Llama via Google Cloud |

Adding a provider is a single Go file implementing the `Provider` interface.

---

## 🔧 Built-in Tools (20+)

| Category | Tools |
|---|---|
| **Web** | WebFetch (HTTP GET with headers) |
| **File** | FileRead, FileWrite |
| **Shell** | ShellExec (commands with timeout) |
| **HTTP** | HTTPRequest (any method/headers/body) |
| **Code** | PythonExec, JavaScriptExec, GoEval |
| **Data** | CSVRead, TextSearch, TextReplace, Base64Encode, Base64Decode, Hash (SHA256/MD5) |
| **System** | ListFiles, DiskUsage, Env, Timestamp |
| **Utility** | JSONParse, Wait |

All tools execute with per-tool timeouts and return structured `ToolResult` with success/error/artifact metadata.

---

## 🔌 Connections

| Type | Description |
|---|---|
| **MCP Server** | JSON-RPC server for tool integration |
| **WebSocket Bridge** | Polling bridge for real-time clients |
| **Webhooks** | Sender/receiver with HMAC signing |
| **File Watcher** | Polling file system watcher |

All connections are managed by the unified `ConnectionManager` and exposed via the API.

---

## 🛡 Security Model

- Rootless containers where practical
- Per-agent filesystem boundaries (read-only or read-write policies)
- Browser and tool workers isolated in separate containers
- Docker socket never exposed to agent code
- Per-agent CPU, memory, and timeout limits
- Network policy options: `full`, `restricted`, `none`
- Full audit trail for tool and browser actions

---

## 📋 Operator Scripts

```bash
./scripts/install.sh              # Full stack setup on a fresh server
./scripts/update.sh               # Safe update with backup + migration
./scripts/doctor.sh               # 10-point health check
./scripts/backup.sh               # SQLite + config + workspace archival
./scripts/scale-agents.sh --count 100  # Bulk create test agents
```

---

## 🗺 Roadmap

### Phase 0–1 ✅
- [x] Architecture ADRs
- [x] Go control plane with SQLite storage
- [x] Docker Compose stack + operator scripts
- [x] Agent CRUD via API and CLI
- [x] 13 provider adapters
- [x] Playwright browser worker
- [x] Scheduler with event-driven wakeup
- [x] Health checks and telemetry

### Phase 2–3 ✅
- [x] Agent templates and policy presets
- [x] Sub-agent spawning system
- [x] 20+ built-in tools (web, file, code, data, system)
- [x] MCP / WebSocket / webhook / file watcher connections
- [x] Fleet management dashboard API

### Phase 4–6 (Current)
- [ ] Per-agent workspace management
- [ ] Cron-based schedules with jitter
- [ ] Active-hours windows
- [ ] Browser pooling with strict concurrency
- [ ] Conversation summarization
- [ ] Artifact retention and cleanup

### Phase 7–9
- [ ] Frontend dashboard UI
- [ ] Benchmark harness (10/50/100/300 agents)
- [ ] Density tuning and idle measurement
- [ ] Multi-node support (optional Postgres backend)

---

## 🧪 Development

```bash
# Dev mode with hot reload
docker compose -f docker/compose.yaml -f docker/compose.dev.yaml up

# Build binaries locally
go build ./cmd/dockclawd
go build ./cmd/dockclawctl

# Run health check
curl http://localhost:8081/health

# Create a test agent
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{"name":"test","provider":{"provider_id":"openai","model":"gpt-4o-mini"}}'
```

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
