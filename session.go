package dashscope

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/google/uuid"

	internalauth "github.com/GizClaw/dashscope-realtime-go/internal/auth"
	internalproto "github.com/GizClaw/dashscope-realtime-go/internal/protocol/dashscope"
	transportws "github.com/GizClaw/dashscope-realtime-go/internal/transport/websocket"
)

// RealtimeService provides realtime session operations.
type RealtimeService struct {
	client *Client
}

// RealtimeSession represents an active realtime websocket session.
type RealtimeSession struct {
	stream *stream
	config *RealtimeConfig
	client *Client

	closeOnce sync.Once
	closeCh   chan struct{}
	doneCh    chan struct{}

	eventsCh chan eventOrError

	mu        sync.RWMutex
	sessionID string
}

type eventOrError struct {
	event *RealtimeEvent
	err   error
}

// Connect opens a realtime session.
func (s *RealtimeService) Connect(ctx context.Context, config *RealtimeConfig) (*RealtimeSession, error) {
	if config == nil {
		config = &RealtimeConfig{}
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = ModelQwenOmniTurboRealtimeLatest
	}

	realtimeURL, err := buildRealtimeURL(s.client.config.baseURL, model)
	if err != nil {
		return nil, err
	}

	headers, err := internalauth.BuildHeaders(s.client.config.apiKey, s.client.config.workspaceID)
	if err != nil {
		if errors.Is(err, internalauth.ErrEmptyAPIKey) {
			return nil, &Error{
				Code:       ErrCodeInvalidAPIKey,
				Message:    err.Error(),
				HTTPStatus: http.StatusUnauthorized,
			}
		}
		return nil, &Error{
			Code:       ErrCodeInvalidParameter,
			Message:    err.Error(),
			HTTPStatus: http.StatusBadRequest,
		}
	}

	wsConn, err := transportws.Dial(ctx, transportws.Config{
		URL:               realtimeURL,
		Headers:           headers,
		HandshakeTimeout:  s.client.config.connectTimeout,
		ReadTimeout:       s.client.config.readTimeout,
		ReadLimitBytes:    s.client.config.readLimitBytes,
		WriteTimeout:      s.client.config.writeTimeout,
		ReconnectAttempts: s.client.config.reconnectAttempts,
		Backoff: transportws.BackoffPolicy{
			BaseDelay: s.client.config.reconnectBase,
			MaxDelay:  s.client.config.reconnectMax,
			Factor:    2,
		},
	})
	if err != nil {
		return nil, mapConnectError(err)
	}

	session := &RealtimeSession{
		stream:   newStream(wsConn),
		config:   &RealtimeConfig{Model: model},
		client:   s.client,
		closeCh:  make(chan struct{}),
		doneCh:   make(chan struct{}),
		eventsCh: make(chan eventOrError, 128),
	}

	go session.readLoop()

	return session, nil
}

// UpdateSession sends session.update event.
func (s *RealtimeSession) UpdateSession(config *SessionConfig) error {
	if config == nil {
		return newInvalidParameterError("session config is required")
	}

	payload := internalproto.SessionUpdatePayload{
		Modalities:        append([]string(nil), config.Modalities...),
		Voice:             strings.TrimSpace(config.Voice),
		InputAudioFormat:  strings.TrimSpace(config.InputAudioFormat),
		OutputAudioFormat: strings.TrimSpace(config.OutputAudioFormat),
		Instructions:      config.Instructions,
		Temperature:       config.Temperature,
		MaxOutputTokens:   config.MaxOutputTokens,
	}

	if config.EnableInputAudioTranscription {
		payload.InputAudioTranscription = &internalproto.InputAudioTranscriptionPayload{
			Model: strings.TrimSpace(config.InputAudioTranscriptionModel),
		}
	}

	if config.TurnDetection != nil {
		payload.TurnDetection = &internalproto.TurnDetectionPayload{
			Type:              strings.TrimSpace(config.TurnDetection.Type),
			PrefixPaddingMs:   config.TurnDetection.PrefixPaddingMs,
			SilenceDurationMs: config.TurnDetection.SilenceDurationMs,
			Threshold:         config.TurnDetection.Threshold,
		}
	}

	event := internalproto.SessionUpdateEvent(generateEventID(), payload)
	return s.sendEvent(context.Background(), event)
}

// AppendAudio sends raw audio bytes as base64 to input audio buffer.
func (s *RealtimeSession) AppendAudio(audio []byte) error {
	if len(audio) == 0 {
		return newInvalidParameterError("audio frame cannot be empty")
	}
	encoded := base64.StdEncoding.EncodeToString(audio)
	event := internalproto.InputAudioAppendEvent(generateEventID(), encoded)
	return s.sendEvent(context.Background(), event)
}

// AppendAudioBase64 sends pre-encoded audio bytes.
func (s *RealtimeSession) AppendAudioBase64(audioBase64 string) error {
	audioBase64 = strings.TrimSpace(audioBase64)
	if audioBase64 == "" {
		return newInvalidParameterError("audio frame cannot be empty")
	}
	event := internalproto.InputAudioAppendEvent(generateEventID(), audioBase64)
	return s.sendEvent(context.Background(), event)
}

// AppendText sends text input.
func (s *RealtimeSession) AppendText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return newInvalidParameterError("text input cannot be empty")
	}
	event := internalproto.InputTextAppendEvent(generateEventID(), text)
	return s.sendEvent(context.Background(), event)
}

// AppendImage sends image frame as base64.
func (s *RealtimeSession) AppendImage(image []byte) error {
	if len(image) == 0 {
		return newInvalidParameterError("image frame cannot be empty")
	}
	encoded := base64.StdEncoding.EncodeToString(image)
	event := internalproto.InputImageAppendEvent(generateEventID(), encoded)
	return s.sendEvent(context.Background(), event)
}

// CommitInput commits pending audio buffer.
func (s *RealtimeSession) CommitInput() error {
	event := internalproto.InputAudioCommitEvent(generateEventID())
	return s.sendEvent(context.Background(), event)
}

// CommitAudio is an alias of CommitInput.
func (s *RealtimeSession) CommitAudio() error {
	return s.CommitInput()
}

// ClearInput clears pending input audio buffer.
func (s *RealtimeSession) ClearInput() error {
	event := internalproto.InputAudioClearEvent(generateEventID())
	return s.sendEvent(context.Background(), event)
}

// CreateResponse sends response.create event.
func (s *RealtimeSession) CreateResponse(opts *ResponseCreateOptions) error {
	payload := internalproto.ResponseCreatePayload{}
	if opts != nil {
		if len(opts.Messages) > 0 {
			messages := make([]internalproto.SimpleMessage, 0, len(opts.Messages))
			for _, msg := range opts.Messages {
				if strings.TrimSpace(msg.Content) == "" {
					return newInvalidParameterError("message content cannot be empty")
				}
				messages = append(messages, internalproto.SimpleMessage{
					Role:    strings.TrimSpace(msg.Role),
					Content: msg.Content,
				})
			}
			payload.Messages = messages
		}

		if opts.Instructions != "" || len(opts.Modalities) > 0 {
			payload.Response = &internalproto.ResponseOptionsPayload{
				Instructions: opts.Instructions,
				Modalities:   append([]string(nil), opts.Modalities...),
			}
		}
	}

	event := internalproto.ResponseCreateEvent(generateEventID(), payload)
	return s.sendEvent(context.Background(), event)
}

// CancelResponse sends response.cancel event.
func (s *RealtimeSession) CancelResponse() error {
	event := internalproto.ResponseCancelEvent(generateEventID())
	return s.sendEvent(context.Background(), event)
}

// FinishSession gracefully closes session by sending session.finish.
func (s *RealtimeSession) FinishSession() error {
	event := internalproto.SessionFinishEvent(generateEventID())
	return s.sendEvent(context.Background(), event)
}

// SendRaw sends custom event payload.
func (s *RealtimeSession) SendRaw(event map[string]any) error {
	if len(event) == 0 {
		return newInvalidParameterError("event payload cannot be empty")
	}
	return s.sendEvent(context.Background(), event)
}

// Events returns an iterator over server events.
func (s *RealtimeSession) Events() iter.Seq2[*RealtimeEvent, error] {
	return func(yield func(*RealtimeEvent, error) bool) {
		for item := range s.eventsCh {
			if !yield(item.event, item.err) {
				return
			}
			if item.err != nil {
				return
			}
		}
	}
}

// Close closes the session.
func (s *RealtimeSession) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		close(s.closeCh)
		closeErr = s.stream.close("client closed")
		<-s.doneCh
	})
	return closeErr
}

// SessionID returns server assigned session ID.
func (s *RealtimeSession) SessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionID
}

func (s *RealtimeSession) sendEvent(ctx context.Context, event map[string]any) error {
	if s.isClosed() {
		return ErrSessionClosed
	}

	if err := s.stream.send(ctx, event); err != nil {
		var sendErr *transportSendError
		if !errors.As(err, &sendErr) {
			return fmt.Errorf("send event: %w", err)
		}

		if !transportws.IsRetryable(sendErr.Unwrap()) {
			return fmt.Errorf("send event: %w", err)
		}

		if reconnectErr := s.stream.reconnect(ctx); reconnectErr != nil {
			return fmt.Errorf("send event reconnect: %w", reconnectErr)
		}
		if retryErr := s.stream.send(ctx, event); retryErr != nil {
			return fmt.Errorf("send event retry: %w", retryErr)
		}
	}

	return nil
}

func (s *RealtimeSession) readLoop() {
	defer close(s.doneCh)
	defer close(s.eventsCh)

	for {
		if s.isClosed() {
			return
		}

		message, err := s.stream.recv(context.Background())
		if err != nil {
			if s.isClosed() {
				return
			}

			if transportws.IsRetryable(err) {
				if reconnectErr := s.stream.reconnect(context.Background()); reconnectErr == nil {
					continue
				} else {
					s.pushErr(fmt.Errorf("read loop reconnect failed: %w", reconnectErr))
					return
				}
			}

			s.pushErr(fmt.Errorf("read loop: %w", err))
			return
		}

		decoded, err := internalproto.DecodeServerEvent(message)
		if err != nil {
			s.pushErr(fmt.Errorf("decode event: %w", err))
			return
		}

		event := convertWireEvent(decoded)
		if event == nil {
			continue
		}

		if event.Type == EventTypeSessionCreated && event.Session != nil && event.Session.ID != "" {
			s.mu.Lock()
			s.sessionID = event.Session.ID
			s.mu.Unlock()
		}

		s.pushEvent(event)
	}
}

func (s *RealtimeSession) pushEvent(event *RealtimeEvent) {
	select {
	case <-s.closeCh:
		return
	case s.eventsCh <- eventOrError{event: event}:
	}
}

func (s *RealtimeSession) pushErr(err error) {
	select {
	case <-s.closeCh:
		return
	case s.eventsCh <- eventOrError{err: err}:
	}
}

func (s *RealtimeSession) isClosed() bool {
	select {
	case <-s.closeCh:
		return true
	default:
		return false
	}
}

func generateEventID() string {
	return "event_" + uuid.New().String()[:12]
}

func buildRealtimeURL(baseURL, model string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", newInvalidParameterError("base URL cannot be empty")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", newInvalidParameterError("invalid base URL")
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return "", newInvalidParameterError("base URL must use ws or wss scheme")
	}

	query := parsed.Query()
	query.Set("model", model)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

func mapConnectError(err error) error {
	var connectErr *transportws.ConnectError
	if !errors.As(err, &connectErr) {
		return &Error{
			Code:       ErrCodeConnectionFailed,
			Message:    err.Error(),
			HTTPStatus: http.StatusServiceUnavailable,
		}
	}

	status := connectErr.StatusCode
	if status <= 0 {
		status = http.StatusServiceUnavailable
	}

	message := strings.TrimSpace(connectErr.Body)
	if message == "" {
		message = connectErr.Error()
	}

	return &Error{
		Code:       mapHTTPStatusToErrorCode(status),
		Message:    message,
		HTTPStatus: status,
	}
}
