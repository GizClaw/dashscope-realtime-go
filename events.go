package dashscope

// Event types for realtime communication.
const (
	// Client events.
	EventTypeSessionUpdate       = "session.update"
	EventTypeInputAudioAppend    = "input_audio_buffer.append"
	EventTypeInputAudioCommit    = "input_audio_buffer.commit"
	EventTypeInputAudioClear     = "input_audio_buffer.clear"
	EventTypeInputTextAppend     = "input_text_buffer.append"
	EventTypeResponseCreate      = "response.create"
	EventTypeResponseCancel      = "response.cancel"
	EventTypeSessionFinish       = "session.finish"
	EventTypeTranscriptionUpdate = "transcription.update"

	// Server events.
	EventTypeSessionCreated                     = "session.created"
	EventTypeSessionUpdated                     = "session.updated"
	EventTypeInputAudioCommitted                = "input_audio_buffer.committed"
	EventTypeInputAudioCleared                  = "input_audio_buffer.cleared"
	EventTypeInputSpeechStarted                 = "input_audio_buffer.speech_started"
	EventTypeInputSpeechStopped                 = "input_audio_buffer.speech_stopped"
	EventTypeResponseCreated                    = "response.created"
	EventTypeResponseDone                       = "response.done"
	EventTypeResponseOutputAdded                = "response.output_item.added"
	EventTypeResponseOutputDone                 = "response.output_item.done"
	EventTypeResponseContentAdded               = "response.content_part.added"
	EventTypeResponseContentDone                = "response.content_part.done"
	EventTypeResponseTextDelta                  = "response.text.delta"
	EventTypeResponseTextDone                   = "response.text.done"
	EventTypeResponseAudioDelta                 = "response.audio.delta"
	EventTypeResponseAudioDone                  = "response.audio.done"
	EventTypeResponseTranscriptDelta            = "response.audio_transcript.delta"
	EventTypeResponseTranscriptDone             = "response.audio_transcript.done"
	EventTypeResponseFunctionCallArgumentsDelta = "response.function_call_arguments.delta"
	EventTypeResponseFunctionCallArgumentsDone  = "response.function_call_arguments.done"
	EventTypeInputAudioTranscriptionCompleted   = "conversation.item.input_audio_transcription.completed"
	EventTypeError                              = "error"

	// DashScope-specific compact response format.
	EventTypeChoicesResponse = "choices"
)

// RealtimeEvent represents one server event.
type RealtimeEvent struct {
	Type string `json:"type"`

	EventID string `json:"event_id,omitempty"`

	RequestID string `json:"request_id,omitempty"`

	LogID string `json:"log_id,omitempty"`

	TraceID string `json:"trace_id,omitempty"`

	Session *SessionInfo `json:"session,omitempty"`

	Response *ResponseInfo `json:"response,omitempty"`

	ResponseID string `json:"response_id,omitempty"`

	Delta string `json:"delta,omitempty"`

	Audio []byte `json:"-"`

	AudioBase64 string `json:"audio,omitempty"`

	Transcript string `json:"transcript,omitempty"`

	FinishReason string `json:"finish_reason,omitempty"`

	ItemID string `json:"item_id,omitempty"`

	OutputIndex int `json:"output_index,omitempty"`

	ContentIndex int `json:"content_index,omitempty"`

	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	Error *EventError `json:"error,omitempty"`

	Usage *UsageStats `json:"usage,omitempty"`
}

// ResponseInfo contains response status details.
type ResponseInfo struct {
	ID           string        `json:"id,omitempty"`
	RequestID    string        `json:"request_id,omitempty"`
	LogID        string        `json:"log_id,omitempty"`
	TraceID      string        `json:"trace_id,omitempty"`
	Status       string        `json:"status,omitempty"`
	StatusDetail *StatusDetail `json:"status_detail,omitempty"`
	Output       []OutputItem  `json:"output,omitempty"`
	Usage        *UsageStats   `json:"usage,omitempty"`
}

// StatusDetail contains reason and nested error.
type StatusDetail struct {
	Type   string `json:"type,omitempty"`
	Reason string `json:"reason,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

// OutputItem is one output unit in a response.
type OutputItem struct {
	ID        string        `json:"id,omitempty"`
	Type      string        `json:"type,omitempty"`
	Role      string        `json:"role,omitempty"`
	Status    string        `json:"status,omitempty"`
	Content   []ContentPart `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
}

// ContentPart is one output content part.
type ContentPart struct {
	Type       string `json:"type,omitempty"`
	Text       string `json:"text,omitempty"`
	Audio      string `json:"audio,omitempty"`
	Transcript string `json:"transcript,omitempty"`
}

// EventError is a realtime error payload.
type EventError struct {
	Type      string `json:"type,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	LogID     string `json:"log_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}
