# dashscope-realtime-go

[![CI](https://github.com/GizClaw/dashscope-realtime-go/actions/workflows/ci.yml/badge.svg)](https://github.com/GizClaw/dashscope-realtime-go/actions/workflows/ci.yml)
[![Go Report: A+](https://img.shields.io/badge/Go%20Report-A%2B-brightgreen)](https://goreportcard.com/report/github.com/GizClaw/dashscope-realtime-go)
[![Code Scan: A](https://img.shields.io/badge/Code%20Scan-A-brightgreen)](https://github.com/GizClaw/dashscope-realtime-go/security/code-scanning)

A lightweight Go SDK for the DashScope Realtime API.

This repository focuses on realtime capabilities only (text/audio streaming and typed Function Calling) and keeps a simple public API in the root package, with protocol and transport details isolated internally.

---

## Requirements

- Go `1.26+`
- A valid `DASHSCOPE_API_KEY`

---

## Installation

```bash
go get github.com/GizClaw/dashscope-realtime-go
```

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    dashscope "github.com/GizClaw/dashscope-realtime-go"
)

func main() {
    apiKey := os.Getenv("DASHSCOPE_API_KEY")
    client := dashscope.NewClient(apiKey)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    session, err := client.Realtime.Connect(ctx, &dashscope.RealtimeConfig{
        Model: dashscope.ModelQwenOmniTurboRealtimeLatest,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer session.Close()

    // Optional: set text mode for this session.
    _ = session.UpdateSession(&dashscope.SessionConfig{
        Modalities: []string{dashscope.ModalityText},
    })

    _ = session.CreateResponse(&dashscope.ResponseCreateOptions{
        Instructions: "Say hello in one short sentence.",
        Modalities:   []string{dashscope.ModalityText},
    })

    for event, evErr := range session.Events() {
        if evErr != nil {
            log.Fatal(evErr)
        }
        if event == nil {
            continue
        }
        if event.Delta != "" {
            fmt.Print(event.Delta)
        }
        if event.Type == dashscope.EventTypeResponseDone {
            break
        }
    }
}
```

---

## Examples

### Quickstart

```bash
go run ./examples/quickstart
```

### Text chat

```bash
go run ./examples/text-chat -rounds 1 -prompt "Hello"
```

### Audio stream

```bash
go run ./examples/audio-stream -rounds 1
```

### Function Calling

```bash
go run ./examples/function-calling
```

Function Calling is supported by the Qwen3.5 Omni Realtime model families. It is not supported by Qwen3 Omni Flash Realtime or legacy Qwen Omni Turbo Realtime models. The provider does not support `tool_choice` or `parallel_tool_calls` for Qwen Omni Realtime, and Function Calling cannot be enabled together with provider web search.

Register tools through `SessionConfig.Tools`. Execute calls only after receiving `EventTypeResponseFunctionCallArgumentsDone`, submit the matching result, then explicitly create the follow-up response:

```go
if event.Type == dashscope.EventTypeResponseFunctionCallArgumentsDone {
    result := `{"ok":true}`
    if err := session.SubmitFunctionCallOutput(event.CallID, result); err != nil {
        return err
    }
    if err := session.CreateResponse(nil); err != nil {
        return err
    }
}
```

The SDK transports declarations, calls, and results. The application remains responsible for tool lookup, authorization, execution, retries, and result generation. `SendRaw` remains available for protocol fields not yet exposed as typed API.

Useful environment variables:

- `DASHSCOPE_API_KEY` (required)
- `DASHSCOPE_MODEL` (optional)
- `DASHSCOPE_BASE_URL` (optional)
- `DASHSCOPE_AUDIO_FILE` (optional, for audio example)

---

## Error Handling

Public API errors are exposed as `*dashscope.Error`.

```go
if apiErr, ok := dashscope.AsError(err); ok {
    fmt.Println(apiErr.Code, apiErr.HTTPStatus, apiErr.Message)
    fmt.Println(apiErr.RequestID, apiErr.LogID, apiErr.TraceID)
}
```

Realtime events also expose debugging identifiers when DashScope returns them:

```go
if event.Error != nil {
    log.Printf("code=%s request_id=%s log_id=%s trace_id=%s",
        event.Error.Code, event.Error.RequestID, event.Error.LogID, event.Error.TraceID)
}
```

Common error groups:

- Authentication (`InvalidApiKey`, `AccessDenied`)
- Rate limit / quota (`RateLimitExceeded`, `QuotaExceeded`)
- Transport (`ConnectionFailed`)

---

## Development

Build:

```bash
go build ./...
```

Test:

```bash
go test ./...
```

Run one test:

```bash
go test . -run '^TestRealtimeSessionAuthFailure$' -count=1
```

---

## CI

The CI workflow runs on push and pull requests:

1. `go build ./...`
2. `go test ./...`

---

## License

Please refer to the repository license file once published.
