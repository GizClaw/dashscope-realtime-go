package dashscope

import (
	"context"
	"encoding/base64"
	"fmt"

	internalproto "github.com/GizClaw/dashscope-realtime-go/internal/protocol/dashscope"
	transportws "github.com/GizClaw/dashscope-realtime-go/internal/transport/websocket"
)

type stream struct {
	conn *transportws.Conn
}

type transportSendError struct {
	err error
}

func (e *transportSendError) Error() string {
	if e == nil || e.err == nil {
		return "<nil>"
	}
	return e.err.Error()
}

func (e *transportSendError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newStream(conn *transportws.Conn) *stream {
	return &stream{conn: conn}
}

func (s *stream) send(ctx context.Context, event map[string]any) error {
	payload, err := internalproto.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}

	if err := s.conn.Write(ctx, payload); err != nil {
		return &transportSendError{err: err}
	}

	return nil
}

func (s *stream) recv(ctx context.Context) ([]byte, error) {
	return s.conn.Read(ctx)
}

func (s *stream) reconnect(ctx context.Context) error {
	return s.conn.Reconnect(ctx)
}

func (s *stream) close(reason string) error {
	return s.conn.Close(reason)
}

func convertWireEvent(w *internalproto.WireEvent) *RealtimeEvent {
	if w == nil {
		return nil
	}

	event := &RealtimeEvent{
		Type:         w.Type,
		EventID:      w.EventID,
		RequestID:    w.RequestID,
		LogID:        w.LogID,
		TraceID:      w.TraceID,
		ResponseID:   w.ResponseID,
		ItemID:       w.ItemID,
		OutputIndex:  w.OutputIndex,
		ContentIndex: w.ContentIndex,
		Delta:        w.Delta,
		AudioBase64:  w.AudioBase64,
		Transcript:   w.Transcript,
		FinishReason: w.FinishReason,
	}

	if w.Session != nil {
		event.Session = &SessionInfo{
			ID:                w.Session.ID,
			Model:             w.Session.Model,
			Modalities:        w.Session.Modalities,
			Voice:             w.Session.Voice,
			InputAudioFormat:  w.Session.InputAudioFormat,
			OutputAudioFormat: w.Session.OutputAudioFormat,
			Instructions:      w.Session.Instructions,
			Temperature:       w.Session.Temperature,
			MaxOutputTokens:   w.Session.MaxOutputTokens,
		}
		if w.Session.TurnDetection != nil {
			event.Session.TurnDetection = &TurnDetection{
				Type:              w.Session.TurnDetection.Type,
				PrefixPaddingMs:   w.Session.TurnDetection.PrefixPaddingMs,
				SilenceDurationMs: w.Session.TurnDetection.SilenceDurationMs,
				Threshold:         w.Session.TurnDetection.Threshold,
			}
		}
	}

	if w.Response != nil {
		event.Response = convertResponseInfo(w.Response)
		if event.RequestID == "" {
			event.RequestID = w.Response.RequestID
		}
		if event.LogID == "" {
			event.LogID = w.Response.LogID
		}
		if event.TraceID == "" {
			event.TraceID = w.Response.TraceID
		}
		if event.ResponseID == "" {
			event.ResponseID = event.Response.ID
		}
		if event.Usage == nil && event.Response.Usage != nil {
			event.Usage = event.Response.Usage
		}
	}

	if w.Item != nil {
		if event.ItemID == "" {
			event.ItemID = w.Item.ID
		}
		if event.Response == nil {
			event.Response = &ResponseInfo{}
		}
		event.Response.Output = append(event.Response.Output, convertOutputItem(w.Item))
	}

	if w.Part != nil {
		if event.Delta == "" {
			if w.Part.Text != "" {
				event.Delta = w.Part.Text
			} else if w.Part.Transcript != "" {
				event.Delta = w.Part.Transcript
			}
		}
		if event.AudioBase64 == "" && w.Part.Audio != "" {
			event.AudioBase64 = w.Part.Audio
		}
	}

	if w.Usage != nil {
		event.Usage = &UsageStats{
			TotalTokens:  w.Usage.TotalTokens,
			InputTokens:  w.Usage.InputTokens,
			OutputTokens: w.Usage.OutputTokens,
		}
	}

	if w.Error != nil {
		event.Error = &EventError{
			Type:      w.Error.Type,
			Code:      w.Error.Code,
			Message:   w.Error.Message,
			Param:     w.Error.Param,
			RequestID: w.Error.RequestID,
			LogID:     w.Error.LogID,
			TraceID:   w.Error.TraceID,
		}
		if event.RequestID == "" {
			event.RequestID = w.Error.RequestID
		}
		if event.LogID == "" {
			event.LogID = w.Error.LogID
		}
		if event.TraceID == "" {
			event.TraceID = w.Error.TraceID
		}
	}

	if event.AudioBase64 != "" {
		if decoded, err := base64.StdEncoding.DecodeString(event.AudioBase64); err == nil {
			event.Audio = decoded
		}
	}

	return event
}

func convertResponseInfo(in *internalproto.ResponseData) *ResponseInfo {
	if in == nil {
		return nil
	}

	out := &ResponseInfo{
		ID:        in.ID,
		RequestID: in.RequestID,
		LogID:     in.LogID,
		TraceID:   in.TraceID,
		Status:    in.Status,
	}

	if in.StatusDetail != nil {
		out.StatusDetail = &StatusDetail{
			Type:   in.StatusDetail.Type,
			Reason: in.StatusDetail.Reason,
		}
		if in.StatusDetail.Error != nil {
			out.StatusDetail.Error = &Error{
				Code:      in.StatusDetail.Error.Code,
				Message:   in.StatusDetail.Error.Message,
				RequestID: in.StatusDetail.Error.RequestID,
				LogID:     in.StatusDetail.Error.LogID,
				TraceID:   in.StatusDetail.Error.TraceID,
			}
		}
	}

	if in.Usage != nil {
		out.Usage = &UsageStats{
			TotalTokens:  in.Usage.TotalTokens,
			InputTokens:  in.Usage.InputTokens,
			OutputTokens: in.Usage.OutputTokens,
		}
	}

	if len(in.Output) > 0 {
		out.Output = make([]OutputItem, 0, len(in.Output))
		for i := range in.Output {
			item := convertOutputItem(&in.Output[i])
			out.Output = append(out.Output, item)
		}
	}

	return out
}

func convertOutputItem(in *internalproto.OutputItemData) OutputItem {
	if in == nil {
		return OutputItem{}
	}

	out := OutputItem{
		ID:     in.ID,
		Type:   in.Type,
		Role:   in.Role,
		Status: in.Status,
	}

	if len(in.Content) > 0 {
		out.Content = make([]ContentPart, 0, len(in.Content))
		for i := range in.Content {
			part := in.Content[i]
			out.Content = append(out.Content, ContentPart{
				Type:       part.Type,
				Text:       part.Text,
				Audio:      part.Audio,
				Transcript: part.Transcript,
			})
		}
	}

	return out
}
