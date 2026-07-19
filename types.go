package dashscope

// Common models for DashScope Realtime API.
const (
	ModelQwenOmniTurboRealtime           = "qwen-omni-turbo-realtime"
	ModelQwenOmniTurboRealtimeLatest     = "qwen-omni-turbo-realtime-latest"
	ModelQwen3OmniFlashRealtime          = "qwen3-omni-flash-realtime"
	ModelQwen3OmniFlashRealtimeLatest    = "qwen3-omni-flash-realtime-latest"
	ModelQwen35OmniPlusRealtime          = "qwen3.5-omni-plus-realtime"
	ModelQwen35OmniPlusRealtime20260315  = "qwen3.5-omni-plus-realtime-2026-03-15"
	ModelQwen35OmniFlashRealtime         = "qwen3.5-omni-flash-realtime"
	ModelQwen35OmniFlashRealtime20260315 = "qwen3.5-omni-flash-realtime-2026-03-15"
)

// Function tool types supported by DashScope Realtime.
const ToolTypeFunction = "function"

// Audio formats supported by DashScope.
const (
	AudioFormatPCM16 = "pcm16"
	AudioFormatPCM24 = "pcm24"
	AudioFormatWAV   = "wav"
	AudioFormatMP3   = "mp3"
)

// Voice IDs for TTS output.
const (
	VoiceChelsie = "Chelsie"
	VoiceCherry  = "Cherry"
	VoiceSerena  = "Serena"
	VoiceEthan   = "Ethan"
)

// VAD modes.
const (
	VADModeServerVAD = "server_vad"
	VADModeDisabled  = "disabled"
)

// Output modalities.
const (
	ModalityText  = "text"
	ModalityAudio = "audio"
)

// RealtimeConfig is the configuration for creating a realtime session.
type RealtimeConfig struct {
	Model string `json:"model,omitempty"`
}

// SessionConfig updates runtime session parameters.
type SessionConfig struct {
	TurnDetection *TurnDetection `json:"turn_detection,omitempty"`

	InputAudioFormat  string   `json:"input_audio_format,omitempty"`
	OutputAudioFormat string   `json:"output_audio_format,omitempty"`
	Voice             string   `json:"voice,omitempty"`
	Modalities        []string `json:"modalities,omitempty"`
	Instructions      string   `json:"instructions,omitempty"`

	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"max_output_tokens,omitempty"`

	EnableInputAudioTranscription bool           `json:"enable_input_audio_transcription,omitempty"`
	InputAudioTranscriptionModel  string         `json:"input_audio_transcription_model,omitempty"`
	Tools                         []FunctionTool `json:"tools,omitempty"`
}

// FunctionTool describes a function available to the realtime model.
type FunctionTool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition describes a model-callable function.
type FunctionDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  *JSONSchema `json:"parameters,omitempty"`
}

// JSONSchema models the JSON Schema subset supported for function parameters.
type JSONSchema struct {
	Type                 string                 `json:"type,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	Enum                 []any                  `json:"enum,omitempty"`
	MinLength            *int                   `json:"minLength,omitempty"`
	MaxLength            *int                   `json:"maxLength,omitempty"`
	Minimum              *float64               `json:"minimum,omitempty"`
	Maximum              *float64               `json:"maximum,omitempty"`
	AnyOf                []*JSONSchema          `json:"anyOf,omitempty"`
}

// TurnDetection configures server VAD parameters.
type TurnDetection struct {
	Type              string  `json:"type,omitempty"`
	PrefixPaddingMs   int     `json:"prefix_padding_ms,omitempty"`
	SilenceDurationMs int     `json:"silence_duration_ms,omitempty"`
	Threshold         float64 `json:"threshold,omitempty"`
}

// SessionInfo contains current session information.
type SessionInfo struct {
	ID                string         `json:"id,omitempty"`
	Model             string         `json:"model,omitempty"`
	Modalities        []string       `json:"modalities,omitempty"`
	Voice             string         `json:"voice,omitempty"`
	InputAudioFormat  string         `json:"input_audio_format,omitempty"`
	OutputAudioFormat string         `json:"output_audio_format,omitempty"`
	TurnDetection     *TurnDetection `json:"turn_detection,omitempty"`
	Instructions      string         `json:"instructions,omitempty"`
	Temperature       float64        `json:"temperature,omitempty"`
	MaxOutputTokens   any            `json:"max_output_tokens,omitempty"`
	Tools             []FunctionTool `json:"tools,omitempty"`
}

// TranscriptItem represents a transcript segment.
type TranscriptItem struct {
	ItemID       string `json:"item_id,omitempty"`
	OutputIndex  int    `json:"output_index,omitempty"`
	ContentIndex int    `json:"content_index,omitempty"`
	Transcript   string `json:"transcript,omitempty"`
}

// UsageStats contains token usage information.
type UsageStats struct {
	TotalTokens  int `json:"total_tokens,omitempty"`
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`

	InputTokenDetails  *TokenDetails `json:"input_token_details,omitempty"`
	OutputTokenDetails *TokenDetails `json:"output_token_details,omitempty"`
}

// TokenDetails contains detailed token breakdown.
type TokenDetails struct {
	TextTokens  int `json:"text_tokens,omitempty"`
	AudioTokens int `json:"audio_tokens,omitempty"`
	ImageTokens int `json:"image_tokens,omitempty"`
}

// ResponseCreateOptions contains options for response.create event.
type ResponseCreateOptions struct {
	Messages     []SimpleMessage
	Instructions string
	Modalities   []string
}

// SimpleMessage is a role-content text message.
type SimpleMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewTextMessage creates a text message object.
func NewTextMessage(role, text string) SimpleMessage {
	return SimpleMessage{Role: role, Content: text}
}
