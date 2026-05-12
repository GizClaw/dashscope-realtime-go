package dashscope

import (
	"testing"

	internalproto "github.com/GizClaw/dashscope-realtime-go/internal/protocol/dashscope"
)

func TestConvertWireEventMapsResponseAndIndexes(t *testing.T) {
	wire := &internalproto.WireEvent{
		Type:         EventTypeResponseOutputAdded,
		ResponseID:   "resp_1",
		ItemID:       "item_1",
		OutputIndex:  2,
		ContentIndex: 3,
		Response: &internalproto.ResponseData{
			ID:     "resp_1",
			Status: "in_progress",
			Output: []internalproto.OutputItemData{
				{
					ID:     "item_1",
					Type:   "message",
					Role:   "assistant",
					Status: "in_progress",
					Content: []internalproto.ContentPartData{
						{Type: "text", Text: "hello"},
					},
				},
			},
		},
	}

	event := convertWireEvent(wire)
	if event == nil {
		t.Fatal("convertWireEvent() got nil")
	}
	if event.Response == nil {
		t.Fatal("event.Response is nil")
	}
	if event.Response.ID != "resp_1" {
		t.Fatalf("event.Response.ID = %q, want %q", event.Response.ID, "resp_1")
	}
	if event.ItemID != "item_1" {
		t.Fatalf("event.ItemID = %q, want %q", event.ItemID, "item_1")
	}
	if event.OutputIndex != 2 {
		t.Fatalf("event.OutputIndex = %d, want 2", event.OutputIndex)
	}
	if event.ContentIndex != 3 {
		t.Fatalf("event.ContentIndex = %d, want 3", event.ContentIndex)
	}
	if got := len(event.Response.Output); got != 1 {
		t.Fatalf("len(event.Response.Output) = %d, want 1", got)
	}
}

func TestConvertWireEventMapsDebugIDs(t *testing.T) {
	wire := &internalproto.WireEvent{
		Type:      EventTypeError,
		RequestID: "req_top",
		LogID:     "log_top",
		TraceID:   "trace_top",
		Error: &internalproto.EventErrorData{
			Code:      "InvalidParameter",
			Message:   "bad input",
			RequestID: "req_error",
			LogID:     "log_error",
			TraceID:   "trace_error",
		},
		Response: &internalproto.ResponseData{
			ID:        "resp_1",
			RequestID: "req_response",
			LogID:     "log_response",
			TraceID:   "trace_response",
			StatusDetail: &internalproto.StatusDetailData{
				Error: &internalproto.EventErrorData{
					Code:      "BadResponse",
					Message:   "response failed",
					RequestID: "req_status",
					LogID:     "log_status",
					TraceID:   "trace_status",
				},
			},
		},
	}

	event := convertWireEvent(wire)
	if event.RequestID != "req_top" {
		t.Fatalf("event.RequestID = %q, want %q", event.RequestID, "req_top")
	}
	if event.LogID != "log_top" {
		t.Fatalf("event.LogID = %q, want %q", event.LogID, "log_top")
	}
	if event.TraceID != "trace_top" {
		t.Fatalf("event.TraceID = %q, want %q", event.TraceID, "trace_top")
	}
	if event.Error == nil {
		t.Fatal("event.Error is nil")
	}
	if event.Error.RequestID != "req_error" {
		t.Fatalf("event.Error.RequestID = %q, want %q", event.Error.RequestID, "req_error")
	}
	if event.Response == nil {
		t.Fatal("event.Response is nil")
	}
	if event.Response.RequestID != "req_response" {
		t.Fatalf("event.Response.RequestID = %q, want %q", event.Response.RequestID, "req_response")
	}
	if event.Response.StatusDetail == nil || event.Response.StatusDetail.Error == nil {
		t.Fatal("event.Response.StatusDetail.Error is nil")
	}
	if event.Response.StatusDetail.Error.RequestID != "req_status" {
		t.Fatalf("event.Response.StatusDetail.Error.RequestID = %q, want %q", event.Response.StatusDetail.Error.RequestID, "req_status")
	}
}
