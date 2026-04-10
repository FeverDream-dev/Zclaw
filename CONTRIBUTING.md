# Contributing to ZClaw

## Development Setup

```bash
git clone https://github.com/FeverDream-dev/Zclaw.git
cd Zclaw
go mod download
```

## Code Structure

- `cmd/dockclawd/` — Control plane daemon
- `cmd/dockclawctl/` — CLI tool
- `internal/` — Core packages (not importable externally)
- `browser-worker/` — Node/Playwright sidecar
- `scripts/` — Operator shell scripts

## Adding a Provider

1. Create `internal/providers/adapters/<name>.go`
2. Implement the `providers.Provider` interface
3. Register in `cmd/dockclawd/main.go`
4. Add example config in `examples/providers/`

## Adding a Worker Type

1. Define worker config in `internal/runtime/`
2. Add Dockerfile in `docker/images/<type>/`
3. Wire into scheduler in `cmd/dockclawd/main.go`

## Style

- Go 1.24+ standard formatting (`gofmt`)
- No `as any`, `@ts-ignore` equivalents
- No empty catch blocks
- No global state or `init()`
- Use `context.Context` everywhere
- Exported symbols need GoDoc comments

## PR Process

1. Fork → branch → PR
2. Ensure `go build ./...` and `go vet ./...` pass
3. Describe the change and why
4. One logical change per PR
