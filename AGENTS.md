# AGENTS.md
Guide for coding agents working in this repository.

## 1) Repository overview
- Module: `github.com/GizClaw/dashscope-realtime-go`
- Go version: `1.26`
- Public package: root package `dashscope`
- Internal packages:
  - `internal/auth`
  - `internal/protocol/dashscope`
  - `internal/transport/websocket`
- Examples:
  - `examples/quickstart`
  - `examples/text-chat`
  - `examples/audio-stream`

## 2) Cursor / Copilot rules status
Checked and not found:
- `.cursorrules`
- `.cursor/rules/`
- `.github/copilot-instructions.md`
Therefore, follow this file + current code conventions + CI workflow.

## 3) CI source of truth
Workflow: `.github/workflows/ci.yml`
CI currently runs:
1. `go build ./...`
2. `go test ./...`
If these pass locally, CI should pass too.

## 4) Build / lint / test commands
Run all commands from repository root.

### 4.1 Build
- Build all packages: `go build ./...`
- Build root package only: `go build .`
- Build one package: `go build ./internal/transport/websocket`

### 4.2 Test
- Full tests: `go test ./...`
- Verbose tests: `go test -v ./...`
- Disable test cache: `go test -count=1 ./...`

### 4.3 Single-test commands (important)
- Root package single test:
  - `go test . -run '^TestRealtimeSessionAuthFailure$' -count=1`
- Protocol package single test:
  - `go test ./internal/protocol/dashscope -run '^TestDecodeServerEventChoicesFormat$' -count=1`
- Transport package single test:
  - `go test ./internal/transport/websocket -run '^TestIsRetryableClassification$' -count=1`
- Regex subset in root package:
  - `go test . -run 'TestRealtimeSession.*' -count=1`

### 4.4 Lint / static checks
No linter config file exists yet.
Use baseline checks:
- Format: `gofmt -w .`
- Vet: `go vet ./...`

### 4.5 Coverage / race
- Coverage profile: `go test ./... -coverprofile=coverage.out`
- Coverage summary: `go tool cover -func=coverage.out`
- Race detector: `go test ./... -race`

### 4.6 Run examples
- Quickstart: `go run ./examples/quickstart`
- Text chat: `go run ./examples/text-chat -rounds 1 -prompt "你好"`
- Audio stream: `go run ./examples/audio-stream -rounds 1`
Common environment variables:
- `DASHSCOPE_API_KEY` (required for real API calls)
- `DASHSCOPE_MODEL` (optional)
- `DASHSCOPE_BASE_URL` (optional)
- `DASHSCOPE_AUDIO_FILE` (optional, audio example)

## 5) Code style guidelines

### 5.1 Package boundaries
- Keep stable public API in root `dashscope` package.
- Put non-public implementation in `internal/...`.
- Avoid exposing protocol/transport internals in public types.

### 5.2 Imports
- Use Go import grouping order:
  1) stdlib
  2) third-party
  3) module internal imports
- Use explicit aliases when they improve readability (e.g. `internalproto`, `transportws`, `internalauth`).

### 5.3 Formatting and structure
- Always run `gofmt -w .` before finalizing.
- Prefer small focused functions and early returns.
- Keep control flow readable; avoid unnecessary nesting.

### 5.4 Naming conventions
- Exported: `PascalCase`.
- Unexported: `camelCase`.
- Constructors: `New...`.
- Event constants: `EventType...`.
- Error code constants: `ErrCode...`.
- Option builders: `With...`.

### 5.5 Types and JSON contracts
- Public structs should use explicit JSON tags.
- Use pointer fields when unset-vs-zero matters.
- Keep wire/protocol mapping in `internal/protocol/dashscope`.

### 5.6 Error handling
- Do not swallow errors.
- Wrap with `%w` for context (`fmt.Errorf("...: %w", err)`).
- Prefer `errors.Is` / `errors.As` over string matching.
- Use sentinel/typed errors where classification is needed.
- Public-facing failures should map to `*dashscope.Error` where appropriate.

### 5.7 Context and timeout usage
- Network paths must accept/propagate `context.Context`.
- Respect cancellation/deadlines.
- Keep timeout logic centralized (e.g. helper like `withTimeout`).

### 5.8 Concurrency rules
- Protect shared mutable state with mutex/atomics.
- Close channels exactly once.
- Use `sync.Once` for one-time close/shutdown.
- Ensure goroutines have clear termination conditions.

### 5.9 WebSocket and reconnect behavior
- Use `github.com/coder/websocket` transport.
- Keep reconnect logic in `internal/transport/websocket`.
- Retry transport-retryable errors only; do not retry local encode/validation errors.

### 5.10 Testing conventions
- Tests live near code in `*_test.go` files.
- Name tests as `Test<Behavior>`.
- Keep tests deterministic with bounded timeouts.
- Use precise failure messages (`got` / `want`).

### 5.11 Docs and examples
- If API behavior changes, update examples/docs in same PR.
- Keep examples runnable and realistic.
- Never hardcode API keys or secrets.

### 5.12 Repository hygiene
- Do not commit secrets or local credentials.
- `openteam/` is ignored and should not be runtime dependency.
- Keep commits focused (one logical change when possible).

## 6) Pre-PR checklist for agents
Before opening/updating a PR, run:
1. `gofmt -w .`
2. `go vet ./...`
3. `go build ./...`
4. `go test ./...`
5. Targeted single tests for touched areas (`-run`)
If anything fails, fix root cause instead of bypassing checks.
