package dashscope

import "encoding/json"

// DecodeServerEvent decodes one server-side JSON event.
func DecodeServerEvent(message []byte) (*WireEvent, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(message, &raw); err != nil {
		return nil, err
	}

	event := &WireEvent{}
	_ = unmarshalString(raw["type"], &event.Type)
	_ = unmarshalString(raw["event_id"], &event.EventID)
	_ = unmarshalString(raw["request_id"], &event.RequestID)
	_ = unmarshalString(raw["log_id"], &event.LogID)
	_ = unmarshalString(raw["trace_id"], &event.TraceID)
	_ = unmarshalString(raw["response_id"], &event.ResponseID)
	_ = unmarshalString(raw["item_id"], &event.ItemID)
	_ = unmarshalInt(raw["output_index"], &event.OutputIndex)
	_ = unmarshalInt(raw["content_index"], &event.ContentIndex)
	_ = unmarshalString(raw["call_id"], &event.CallID)
	_ = unmarshalString(raw["name"], &event.Name)
	_ = unmarshalString(raw["arguments"], &event.Arguments)

	if choicesRaw, ok := raw["choices"]; ok {
		if parsed := decodeChoices(choicesRaw); parsed != nil {
			parsed.EventID = event.EventID
			parsed.RequestID = event.RequestID
			parsed.LogID = event.LogID
			parsed.TraceID = event.TraceID
			parsed.ResponseID = event.ResponseID
			return parsed, nil
		}
	}

	if sessionRaw, ok := raw["session"]; ok {
		var s SessionData
		if err := json.Unmarshal(sessionRaw, &s); err == nil {
			event.Session = &s
		}
	}

	if responseRaw, ok := raw["response"]; ok {
		var data ResponseData
		if err := json.Unmarshal(responseRaw, &data); err == nil {
			event.Response = &data
			if event.ResponseID == "" {
				event.ResponseID = data.ID
			}
			if data.Usage != nil {
				event.Usage = cloneUsage(data.Usage)
			}
		}
	}

	if itemRaw, ok := raw["item"]; ok {
		var item OutputItemData
		if err := json.Unmarshal(itemRaw, &item); err == nil {
			event.Item = &item
			if event.ItemID == "" {
				event.ItemID = item.ID
			}
		}
	}

	if partRaw, ok := raw["part"]; ok {
		var part ContentPartData
		if err := json.Unmarshal(partRaw, &part); err == nil {
			event.Part = &part
		}
	}

	if deltaRaw, ok := raw["delta"]; ok {
		_ = unmarshalString(deltaRaw, &event.Delta)
	}
	if transcriptRaw, ok := raw["transcript"]; ok {
		_ = unmarshalString(transcriptRaw, &event.Transcript)
	}
	if audioRaw, ok := raw["audio"]; ok {
		_ = unmarshalString(audioRaw, &event.AudioBase64)
	}

	if event.Part != nil {
		if event.Delta == "" {
			if event.Part.Text != "" {
				event.Delta = event.Part.Text
			} else if event.Part.Transcript != "" {
				event.Delta = event.Part.Transcript
			}
		}
		if event.AudioBase64 == "" && event.Part.Audio != "" {
			event.AudioBase64 = event.Part.Audio
		}
	}

	if event.Item != nil && event.Delta == "" {
		for _, content := range event.Item.Content {
			if content.Text != "" {
				event.Delta += content.Text
			}
		}
	}

	if event.Type == "response.audio.delta" && event.AudioBase64 == "" && event.Delta != "" {
		event.AudioBase64 = event.Delta
		event.Delta = ""
	}

	if errRaw, ok := raw["error"]; ok {
		var e EventErrorData
		if err := json.Unmarshal(errRaw, &e); err == nil {
			event.Error = &e
			if event.Type == "" {
				event.Type = "error"
			}
		}
	}

	return event, nil
}

func decodeChoices(raw json.RawMessage) *WireEvent {
	var choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content []struct {
				Text  string `json:"text,omitempty"`
				Audio *struct {
					Data string `json:"data"`
				} `json:"audio,omitempty"`
			} `json:"content"`
		} `json:"message"`
	}

	if err := json.Unmarshal(raw, &choices); err != nil || len(choices) == 0 {
		return nil
	}

	choice := choices[0]
	result := &WireEvent{
		Type:         "choices",
		FinishReason: choice.FinishReason,
	}

	for _, c := range choice.Message.Content {
		if c.Text != "" {
			result.Delta += c.Text
		}
		if c.Audio != nil && c.Audio.Data != "" && result.AudioBase64 == "" {
			result.AudioBase64 = c.Audio.Data
		}
	}

	return result
}

func cloneUsage(in *UsageData) *UsageData {
	if in == nil {
		return nil
	}
	return &UsageData{
		TotalTokens:  in.TotalTokens,
		InputTokens:  in.InputTokens,
		OutputTokens: in.OutputTokens,
	}
}

func unmarshalString(raw json.RawMessage, dst *string) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

func unmarshalInt(raw json.RawMessage, dst *int) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}
