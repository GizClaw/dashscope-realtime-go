package dashscope

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// Authentication errors.
	ErrCodeInvalidAPIKey     = "InvalidApiKey"
	ErrCodeAccessDenied      = "AccessDenied"
	ErrCodeWorkspaceNotFound = "WorkspaceNotFound"

	// Rate limiting.
	ErrCodeRateLimitExceeded = "RateLimitExceeded"
	ErrCodeQuotaExceeded     = "QuotaExceeded"

	// Request errors.
	ErrCodeInvalidParameter = "InvalidParameter"
	ErrCodeModelNotFound    = "ModelNotFound"

	// Server errors.
	ErrCodeInternalError = "InternalError"
	ErrCodeServiceBusy   = "ServiceBusy"

	// Transport errors.
	ErrCodeConnectionFailed = "ConnectionFailed"
)

var (
	// ErrSessionClosed is returned when operation targets a closed session.
	ErrSessionClosed = errors.New("dashscope: session is closed")
)

// Error represents DashScope API error.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
	LogID      string `json:"log_id,omitempty"`
	TraceID    string `json:"trace_id,omitempty"`
	HTTPStatus int    `json:"-"`
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	details := make([]string, 0, 4)
	if e.RequestID != "" {
		details = append(details, "request_id="+e.RequestID)
	}
	if e.LogID != "" {
		details = append(details, "log_id="+e.LogID)
	}
	if e.TraceID != "" {
		details = append(details, "trace_id="+e.TraceID)
	}
	details = append(details, fmt.Sprintf("http_status=%d", e.HTTPStatus))

	return fmt.Sprintf("dashscope: %s - %s (%s)", e.Code, e.Message, strings.Join(details, ", "))
}

// IsRateLimit checks whether the error is rate-limit related.
func (e *Error) IsRateLimit() bool {
	if e == nil {
		return false
	}
	return e.Code == ErrCodeRateLimitExceeded || e.Code == ErrCodeQuotaExceeded
}

// IsAuth checks whether the error is authentication related.
func (e *Error) IsAuth() bool {
	if e == nil {
		return false
	}
	return e.Code == ErrCodeInvalidAPIKey || e.Code == ErrCodeAccessDenied
}

// IsServerError checks whether the error is server-side.
func (e *Error) IsServerError() bool {
	if e == nil {
		return false
	}
	return e.Code == ErrCodeInternalError || e.Code == ErrCodeServiceBusy
}

// Retryable checks whether the request can be retried.
func (e *Error) Retryable() bool {
	if e == nil {
		return false
	}
	return e.IsRateLimit() || e.IsServerError() || e.Code == ErrCodeConnectionFailed
}

// AsError attempts to cast any error to *Error.
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func newInvalidParameterError(message string) *Error {
	return &Error{
		Code:       ErrCodeInvalidParameter,
		Message:    message,
		HTTPStatus: 400,
	}
}

func mapHTTPStatusToErrorCode(status int) string {
	switch status {
	case 400:
		return ErrCodeInvalidParameter
	case 401:
		return ErrCodeInvalidAPIKey
	case 403:
		return ErrCodeAccessDenied
	case 404:
		return ErrCodeModelNotFound
	case 429:
		return ErrCodeRateLimitExceeded
	case 500:
		return ErrCodeInternalError
	case 502, 503, 504:
		return ErrCodeServiceBusy
	default:
		return ErrCodeConnectionFailed
	}
}
