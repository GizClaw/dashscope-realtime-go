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
