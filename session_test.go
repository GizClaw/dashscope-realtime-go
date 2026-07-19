package dashscope

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	transportws "github.com/GizClaw/dashscope-realtime-go/internal/transport/websocket"
)

func TestRealtimeSessionTextPath(t *testing.T) {
	server := newMockRealtimeServer(t, "valid-key")

	client := NewClient("valid-key",
		WithBaseURL(toWSURL(server.URL)),
		WithConnectTimeout(2*time.Second),
		WithWriteTimeout(2*time.Second),
		WithReadTimeout(5*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(ctx, &RealtimeConfig{Model: ModelQwenOmniTurboRealtimeLatest})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	events, errs := startEventReader(session)

	first := waitEvent(t, events, errs, 2*time.Second)
	if first.Type != EventTypeSessionCreated {
		t.Fatalf("first event type = %q, want %q", first.Type, EventTypeSessionCreated)
	}

	if err := session.AppendText("你好"); err != nil {
		t.Fatalf("AppendText() error = %v", err)
	}
	if err := session.CreateResponse(nil); err != nil {
		t.Fatalf("CreateResponse() error = %v", err)
	}

	var gotDelta strings.Builder
	for {
		event := waitEvent(t, events, errs, 2*time.Second)
		switch event.Type {
		case EventTypeResponseTextDelta:
			gotDelta.WriteString(event.Delta)
		case EventTypeResponseDone:
			if !strings.Contains(gotDelta.String(), "你好") {
				t.Fatalf("response delta = %q, want contains %q", gotDelta.String(), "你好")
			}
			return
		}
	}
}

func TestRealtimeSessionEmptyInputValidation(t *testing.T) {
	server := newMockRealtimeServer(t, "valid-key")

	client := NewClient("valid-key", WithBaseURL(toWSURL(server.URL)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(ctx, &RealtimeConfig{})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	if err := session.AppendText(""); err == nil {
		t.Fatal("AppendText(\"\") expected error, got nil")
	} else if apiErr, ok := AsError(err); !ok || apiErr.Code != ErrCodeInvalidParameter {
		t.Fatalf("AppendText error = %v, want InvalidParameter", err)
	}

	if err := session.AppendAudio(nil); err == nil {
		t.Fatal("AppendAudio(nil) expected error, got nil")
	} else if apiErr, ok := AsError(err); !ok || apiErr.Code != ErrCodeInvalidParameter {
		t.Fatalf("AppendAudio error = %v, want InvalidParameter", err)
	}

	if err := session.UpdateSession(&SessionConfig{Tools: []FunctionTool{{
		Type:     "custom",
		Function: FunctionDefinition{Name: "lookup_weather"},
	}}}); err == nil {
		t.Fatal("UpdateSession() expected invalid tool type error, got nil")
	}
	if err := session.UpdateSession(&SessionConfig{Tools: []FunctionTool{{
		Type:     ToolTypeFunction,
		Function: FunctionDefinition{Name: "  "},
	}}}); err == nil {
		t.Fatal("UpdateSession() expected empty function name error, got nil")
	}
	if err := session.SubmitFunctionCallOutput("  ", "result"); err == nil {
		t.Fatal("SubmitFunctionCallOutput() expected empty call ID error, got nil")
	}
}

func TestRealtimeSessionFunctionCallingWireFlow(t *testing.T) {
	requests := make(chan map[string]any, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket accept: %v", err)
			return
		}
		defer conn.Close(coderws.StatusNormalClosure, "done")
		ctx := context.Background()
		if err := wsjson.Write(ctx, conn, map[string]any{
			"type":    "session.created",
			"session": map[string]any{"id": "sess_function_1"},
		}); err != nil {
			t.Errorf("send session.created: %v", err)
			return
		}
		for {
			var req map[string]any
			if err := wsjson.Read(ctx, conn, &req); err != nil {
				return
			}
			requests <- req
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient("valid-key", WithBaseURL(toWSURL(server.URL)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Realtime.Connect(ctx, &RealtimeConfig{Model: ModelQwen35OmniFlashRealtime})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()
	events, errs := startEventReader(session)
	_ = waitEvent(t, events, errs, 2*time.Second)

	additionalProperties := false
	if err := session.UpdateSession(&SessionConfig{Tools: []FunctionTool{{
		Type: ToolTypeFunction,
		Function: FunctionDefinition{
			Name: "lookup_weather",
			Parameters: &JSONSchema{
				Type:                 "object",
				Properties:           map[string]*JSONSchema{"city": {Type: "string"}},
				Required:             []string{"city"},
				AdditionalProperties: &additionalProperties,
			},
		},
	}}}); err != nil {
		t.Fatalf("UpdateSession() error = %v", err)
	}
	update := waitRequest(t, requests)
	tools := update["session"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %#v, want one tool", tools)
	}
	function := tools[0].(map[string]any)["function"].(map[string]any)
	if function["name"] != "lookup_weather" {
		t.Fatalf("function name = %#v, want trimmed name", function["name"])
	}

	output := "  {\"temperature\":25}\n"
	if err := session.SubmitFunctionCallOutput(" call_1 ", output); err != nil {
		t.Fatalf("SubmitFunctionCallOutput() error = %v", err)
	}
	result := waitRequest(t, requests)
	if result["type"] != "conversation.item.create" {
		t.Fatalf("result type = %#v", result["type"])
	}
	item := result["item"].(map[string]any)
	if item["call_id"] != " call_1 " || item["output"] != output {
		t.Fatalf("function result item = %#v", item)
	}
	select {
	case unexpected := <-requests:
		t.Fatalf("SubmitFunctionCallOutput() sent implicit event: %#v", unexpected)
	case <-time.After(100 * time.Millisecond):
	}

	if err := session.CreateResponse(nil); err != nil {
		t.Fatalf("CreateResponse() error = %v", err)
	}
	response := waitRequest(t, requests)
	if response["type"] != EventTypeResponseCreate {
		t.Fatalf("response type = %#v", response["type"])
	}
}

func TestRealtimeSessionUpdateToolsDistinguishesOmittedAndEmpty(t *testing.T) {
	requests := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket accept: %v", err)
			return
		}
		defer conn.Close(coderws.StatusNormalClosure, "done")
		ctx := context.Background()
		if err := wsjson.Write(ctx, conn, map[string]any{
			"type": "session.created", "session": map[string]any{"id": "sess_tools"},
		}); err != nil {
			t.Errorf("send session.created: %v", err)
			return
		}
		for {
			var req map[string]any
			if err := wsjson.Read(ctx, conn, &req); err != nil {
				return
			}
			requests <- req
		}
	}))
	t.Cleanup(server.Close)

	client := NewClient("valid-key", WithBaseURL(toWSURL(server.URL)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := client.Realtime.Connect(ctx, &RealtimeConfig{})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()
	events, errs := startEventReader(session)
	_ = waitEvent(t, events, errs, 2*time.Second)

	if err := session.UpdateSession(&SessionConfig{}); err != nil {
		t.Fatalf("UpdateSession(omitted tools) error = %v", err)
	}
	omitted := waitRequest(t, requests)["session"].(map[string]any)
	if _, exists := omitted["tools"]; exists {
		t.Fatalf("omitted tools payload = %#v, want no tools field", omitted)
	}

	if err := session.UpdateSession(&SessionConfig{Tools: []FunctionTool{}}); err != nil {
		t.Fatalf("UpdateSession(empty tools) error = %v", err)
	}
	empty := waitRequest(t, requests)["session"].(map[string]any)
	tools, exists := empty["tools"]
	if !exists || len(tools.([]any)) != 0 {
		t.Fatalf("empty tools payload = %#v, want explicit empty array", empty)
	}
}

func waitRequest(t *testing.T, requests <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case request := <-requests:
		return request
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client request")
		return nil
	}
}

func TestRealtimeSessionAuthFailure(t *testing.T) {
	server := newMockRealtimeServer(t, "valid-key")

	client := NewClient("invalid-key",
		WithBaseURL(toWSURL(server.URL)),
		WithConnectTimeout(2*time.Second),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Realtime.Connect(ctx, &RealtimeConfig{})
	if err == nil {
		t.Fatal("Connect() expected auth error, got nil")
	}

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("Connect() error = %T, want *Error", err)
	}
	if !apiErr.IsAuth() {
		t.Fatalf("apiErr.IsAuth() = false, err = %+v", apiErr)
	}
	if apiErr.Code != ErrCodeInvalidAPIKey {
		t.Fatalf("apiErr.Code = %q, want %q", apiErr.Code, ErrCodeInvalidAPIKey)
	}
	if apiErr.RequestID != "req_auth_1" {
		t.Fatalf("apiErr.RequestID = %q, want %q", apiErr.RequestID, "req_auth_1")
	}
	if apiErr.LogID != "log_auth_1" {
		t.Fatalf("apiErr.LogID = %q, want %q", apiErr.LogID, "log_auth_1")
	}
	if apiErr.TraceID != "trace_auth_1" {
		t.Fatalf("apiErr.TraceID = %q, want %q", apiErr.TraceID, "trace_auth_1")
	}
}

func TestRealtimeSessionEmptyAPIKeyFailure(t *testing.T) {
	client := NewClient("")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.Realtime.Connect(ctx, &RealtimeConfig{})
	if err == nil {
		t.Fatal("Connect() expected auth error, got nil")
	}

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("Connect() error = %T, want *Error", err)
	}
	if !apiErr.IsAuth() {
		t.Fatalf("apiErr.IsAuth() = false, err = %+v", apiErr)
	}
	if apiErr.Code != ErrCodeInvalidAPIKey {
		t.Fatalf("apiErr.Code = %q, want %q", apiErr.Code, ErrCodeInvalidAPIKey)
	}
}

func TestMapConnectErrorIncludesDebugIDsFromBodyAndHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Request-Id", "req_header")

	err := mapConnectError(&transportws.ConnectError{
		StatusCode: http.StatusTooManyRequests,
		Headers:    headers,
		Body:       `{"code":"Throttled","message":"slow down","request_id":"req_body","log_id":"log_body","trace_id":"trace_body"}`,
	})

	apiErr, ok := AsError(err)
	if !ok {
		t.Fatalf("mapConnectError() error = %T, want *Error", err)
	}
	if apiErr.Code != "Throttled" {
		t.Fatalf("apiErr.Code = %q, want %q", apiErr.Code, "Throttled")
	}
	if apiErr.Message != "slow down" {
		t.Fatalf("apiErr.Message = %q, want %q", apiErr.Message, "slow down")
	}
	if apiErr.RequestID != "req_header" {
		t.Fatalf("apiErr.RequestID = %q, want %q", apiErr.RequestID, "req_header")
	}
	if apiErr.LogID != "log_body" {
		t.Fatalf("apiErr.LogID = %q, want %q", apiErr.LogID, "log_body")
	}
	if apiErr.TraceID != "trace_body" {
		t.Fatalf("apiErr.TraceID = %q, want %q", apiErr.TraceID, "trace_body")
	}
}

func TestSendRawEncodeErrorDoesNotReconnect(t *testing.T) {
	var connectionCount atomic.Int64
	server := newMockRealtimeServerWithCounter(t, "valid-key", &connectionCount)

	client := NewClient("valid-key",
		WithBaseURL(toWSURL(server.URL)),
		WithReconnect(3, 10*time.Millisecond, 20*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(ctx, &RealtimeConfig{})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	events, errs := startEventReader(session)
	_ = waitEvent(t, events, errs, 2*time.Second) // session.created

	err = session.SendRaw(map[string]any{
		"type": "test.bad",
		"bad":  func() {},
	})
	if err == nil {
		t.Fatal("SendRaw() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "encode event") {
		t.Fatalf("SendRaw() error = %v, want encode error", err)
	}
	if strings.Contains(err.Error(), "reconnect") {
		t.Fatalf("SendRaw() unexpectedly triggered reconnect path: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if got := connectionCount.Load(); got != 1 {
		t.Fatalf("connection count = %d, want 1 (no reconnect)", got)
	}
}

func TestRealtimeSessionDecodeErrorStopsStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "bearer valid-key" {
			http.Error(w, "invalid api key", http.StatusUnauthorized)
			return
		}

		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket accept: %v", err)
			return
		}
		defer conn.Close(coderws.StatusNormalClosure, "done")

		ctx := context.Background()
		_ = wsjson.Write(ctx, conn, map[string]any{
			"type": "session.created",
			"session": map[string]any{
				"id": "sess_test_1",
			},
		})

		_ = conn.Write(ctx, coderws.MessageText, []byte(`{"type":`))
		time.Sleep(100 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	client := NewClient("valid-key", WithBaseURL(toWSURL(server.URL)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session, err := client.Realtime.Connect(ctx, &RealtimeConfig{})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer session.Close()

	events, errs := startEventReader(session)
	_ = waitEvent(t, events, errs, 2*time.Second) // session.created

	select {
	case err := <-errs:
		if err == nil {
			t.Fatal("expected decode error, got nil")
		}
		if !strings.Contains(err.Error(), "decode event") {
			t.Fatalf("error = %v, want decode event", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting decode error")
	}
}

func startEventReader(session *RealtimeSession) (<-chan *RealtimeEvent, <-chan error) {
	events := make(chan *RealtimeEvent, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)
		for event, err := range session.Events() {
			if err != nil {
				errs <- err
				return
			}
			events <- event
		}
	}()

	return events, errs
}

func waitEvent(t *testing.T, events <-chan *RealtimeEvent, errs <-chan error, timeout time.Duration) *RealtimeEvent {
	t.Helper()
	select {
	case err, ok := <-errs:
		if !ok {
			t.Fatal("error channel closed unexpectedly")
		}
		if err != nil {
			t.Fatalf("event stream error: %v", err)
		}
		t.Fatal("received nil error from error channel")
		return nil
	case event, ok := <-events:
		if !ok {
			t.Fatal("event channel closed unexpectedly")
		}
		if event == nil {
			t.Fatal("received nil event")
		}
		return event
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for event after %v", timeout)
		return nil
	}
}

func toWSURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func newMockRealtimeServer(t *testing.T, validAPIKey string) *httptest.Server {
	t.Helper()
	var count atomic.Int64
	return newMockRealtimeServerWithCounter(t, validAPIKey, &count)
}

func newMockRealtimeServerWithCounter(t *testing.T, validAPIKey string, connectionCount *atomic.Int64) *httptest.Server {
	t.Helper()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "bearer "+validAPIKey {
			w.Header().Set("X-Request-Id", "req_auth_1")
			w.Header().Set("X-Log-Id", "log_auth_1")
			w.Header().Set("X-Trace-Id", "trace_auth_1")
			http.Error(w, "invalid api key", http.StatusUnauthorized)
			return
		}

		if connectionCount != nil {
			connectionCount.Add(1)
		}

		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			t.Errorf("websocket accept: %v", err)
			return
		}
		defer conn.Close(coderws.StatusNormalClosure, "done")

		ctx := context.Background()
		if err := wsjson.Write(ctx, conn, map[string]any{
			"type": "session.created",
			"session": map[string]any{
				"id": "sess_test_1",
			},
		}); err != nil {
			t.Errorf("send session.created: %v", err)
			return
		}

		var bufferedText string
		for {
			var req map[string]any
			if err := wsjson.Read(ctx, conn, &req); err != nil {
				return
			}

			kind, _ := req["type"].(string)
			switch kind {
			case EventTypeInputTextAppend:
				if text, ok := req["text"].(string); ok {
					bufferedText += text
				}

			case EventTypeSessionUpdate:
				_ = wsjson.Write(ctx, conn, map[string]any{
					"type": "session.updated",
					"session": map[string]any{
						"id": "sess_test_1",
					},
				})

			case EventTypeResponseCreate:
				_ = wsjson.Write(ctx, conn, map[string]any{
					"type": "response.created",
					"response": map[string]any{
						"id": "resp_test_1",
					},
				})
				delta := "echo:" + bufferedText
				if strings.TrimSpace(bufferedText) == "" {
					delta = "echo:ok"
				}
				_ = wsjson.Write(ctx, conn, map[string]any{
					"type":  "response.text.delta",
					"delta": delta,
				})
				_ = wsjson.Write(ctx, conn, map[string]any{
					"type": "response.done",
					"response": map[string]any{
						"usage": map[string]any{
							"total_tokens":  3,
							"input_tokens":  1,
							"output_tokens": 2,
						},
					},
				})
				bufferedText = ""

			case EventTypeSessionFinish:
				return
			}
		}
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}
