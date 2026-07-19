package dashscope

import "encoding/json"

// Marshal marshals event payload into JSON bytes.
func Marshal(event map[string]any) ([]byte, error) {
	return json.Marshal(event)
}

// SessionUpdateEvent builds session.update payload.
func SessionUpdateEvent(eventID string, payload SessionUpdatePayload) map[string]any {
	event := map[string]any{
		"event_id": eventID,
		"type":     "session.update",
		"session":  payload,
	}
	return event
}

// ConversationItemCreateFunctionOutputEvent builds a function result item.
func ConversationItemCreateFunctionOutputEvent(eventID string, payload FunctionCallOutputPayload) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "conversation.item.create",
		"item":     payload,
	}
}

// InputAudioAppendEvent builds input_audio_buffer.append payload.
func InputAudioAppendEvent(eventID, audioBase64 string) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "input_audio_buffer.append",
		"audio":    audioBase64,
	}
}

// InputTextAppendEvent builds input_text_buffer.append payload.
func InputTextAppendEvent(eventID, text string) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "input_text_buffer.append",
		"text":     text,
	}
}

// InputImageAppendEvent builds input_image_buffer.append payload.
func InputImageAppendEvent(eventID, imageBase64 string) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "input_image_buffer.append",
		"image":    imageBase64,
	}
}

// InputAudioCommitEvent builds input_audio_buffer.commit payload.
func InputAudioCommitEvent(eventID string) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "input_audio_buffer.commit",
	}
}

// InputAudioClearEvent builds input_audio_buffer.clear payload.
func InputAudioClearEvent(eventID string) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "input_audio_buffer.clear",
	}
}

// ResponseCreateEvent builds response.create payload.
func ResponseCreateEvent(eventID string, payload ResponseCreatePayload) map[string]any {
	event := map[string]any{
		"event_id": eventID,
		"type":     "response.create",
	}

	if len(payload.Messages) > 0 {
		event["messages"] = payload.Messages
	}
	if payload.Response != nil {
		event["response"] = payload.Response
	}

	return event
}

// ResponseCancelEvent builds response.cancel payload.
func ResponseCancelEvent(eventID string) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "response.cancel",
	}
}

// SessionFinishEvent builds session.finish payload.
func SessionFinishEvent(eventID string) map[string]any {
	return map[string]any{
		"event_id": eventID,
		"type":     "session.finish",
	}
}
