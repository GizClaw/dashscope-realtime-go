package dashscope

// SessionUpdatePayload is wire payload for session.update.
type SessionUpdatePayload struct {
	Modalities []string `json:"modalities,omitempty"`

	Voice             string   `json:"voice,omitempty"`
	InputAudioFormat  string   `json:"input_audio_format,omitempty"`
	OutputAudioFormat string   `json:"output_audio_format,omitempty"`
	Instructions      string   `json:"instructions,omitempty"`
	Temperature       *float64 `json:"temperature,omitempty"`
	MaxOutputTokens   *int     `json:"max_output_tokens,omitempty"`

	InputAudioTranscription *InputAudioTranscriptionPayload `json:"input_audio_transcription,omitempty"`
	TurnDetection           *TurnDetectionPayload           `json:"turn_detection,omitempty"`
	Tools                   *[]FunctionToolPayload          `json:"tools,omitempty"`
}

type FunctionToolPayload struct {
	Type     string                    `json:"type"`
	Function FunctionDefinitionPayload `json:"function"`
}

type FunctionDefinitionPayload struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Parameters  *JSONSchemaPayload `json:"parameters,omitempty"`
}

type JSONSchemaPayload struct {
	Type                 string                        `json:"type,omitempty"`
	Description          string                        `json:"description,omitempty"`
	Properties           map[string]*JSONSchemaPayload `json:"properties,omitempty"`
	Required             []string                      `json:"required,omitempty"`
	AdditionalProperties *bool                         `json:"additionalProperties,omitempty"`
	Items                *JSONSchemaPayload            `json:"items,omitempty"`
	Enum                 []any                         `json:"enum,omitempty"`
	MinLength            *int                          `json:"minLength,omitempty"`
	MaxLength            *int                          `json:"maxLength,omitempty"`
	Minimum              *float64                      `json:"minimum,omitempty"`
	Maximum              *float64                      `json:"maximum,omitempty"`
	AnyOf                []*JSONSchemaPayload          `json:"anyOf,omitempty"`
}

type FunctionCallOutputPayload struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

// InputAudioTranscriptionPayload configures transcription model.
type InputAudioTranscriptionPayload struct {
	Model string `json:"model,omitempty"`
}

// TurnDetectionPayload is wire VAD config.
type TurnDetectionPayload struct {
	Type              string  `json:"type,omitempty"`
	PrefixPaddingMs   int     `json:"prefix_padding_ms,omitempty"`
	SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
	Threshold         float64 `json:"threshold,omitempty"`
}

// SimpleMessage is wire role/content message.
type SimpleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponseCreatePayload is wire payload for response.create.
type ResponseCreatePayload struct {
	Messages []SimpleMessage         `json:"messages,omitempty"`
	Response *ResponseOptionsPayload `json:"response,omitempty"`
}

// ResponseOptionsPayload contains optional response overrides.
type ResponseOptionsPayload struct {
	Instructions string   `json:"instructions,omitempty"`
	Modalities   []string `json:"modalities,omitempty"`
}

// SessionData is a subset of server session payload.
type SessionData struct {
	ID                string                `json:"id,omitempty"`
	Model             string                `json:"model,omitempty"`
	Modalities        []string              `json:"modalities,omitempty"`
	Voice             string                `json:"voice,omitempty"`
	InputAudioFormat  string                `json:"input_audio_format,omitempty"`
	OutputAudioFormat string                `json:"output_audio_format,omitempty"`
	TurnDetection     *TurnDetectionPayload `json:"turn_detection,omitempty"`
	Instructions      string                `json:"instructions,omitempty"`
	Temperature       float64               `json:"temperature,omitempty"`
	MaxOutputTokens   any                   `json:"max_output_tokens,omitempty"`
	Tools             []FunctionToolPayload `json:"tools,omitempty"`
}

// UsageData is token usage payload.
type UsageData struct {
	TotalTokens  int `json:"total_tokens,omitempty"`
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// ResponseData is response payload from server events.
type ResponseData struct {
	ID           string            `json:"id,omitempty"`
	RequestID    string            `json:"request_id,omitempty"`
	LogID        string            `json:"log_id,omitempty"`
	TraceID      string            `json:"trace_id,omitempty"`
	Status       string            `json:"status,omitempty"`
	StatusDetail *StatusDetailData `json:"status_detail,omitempty"`
	Output       []OutputItemData  `json:"output,omitempty"`
	Usage        *UsageData        `json:"usage,omitempty"`
}

// StatusDetailData contains detailed response status.
type StatusDetailData struct {
	Type   string          `json:"type,omitempty"`
	Reason string          `json:"reason,omitempty"`
	Error  *EventErrorData `json:"error,omitempty"`
}

// OutputItemData is one output item in response payload.
type OutputItemData struct {
	ID        string            `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Role      string            `json:"role,omitempty"`
	Status    string            `json:"status,omitempty"`
	Content   []ContentPartData `json:"content,omitempty"`
	CallID    string            `json:"call_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments string            `json:"arguments,omitempty"`
}

// ContentPartData is one output content part payload.
type ContentPartData struct {
	Type       string `json:"type,omitempty"`
	Text       string `json:"text,omitempty"`
	Audio      string `json:"audio,omitempty"`
	Transcript string `json:"transcript,omitempty"`
}

// EventErrorData is wire error payload.
type EventErrorData struct {
	Type      string `json:"type,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	LogID     string `json:"log_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
}

// WireEvent is decoded server message.
type WireEvent struct {
	Type         string
	EventID      string
	RequestID    string
	LogID        string
	TraceID      string
	Session      *SessionData
	Response     *ResponseData
	Item         *OutputItemData
	Part         *ContentPartData
	ResponseID   string
	ItemID       string
	OutputIndex  int
	ContentIndex int
	CallID       string
	Name         string
	Arguments    string
	Delta        string
	AudioBase64  string
	Transcript   string
	FinishReason string
	Usage        *UsageData
	Error        *EventErrorData
}
