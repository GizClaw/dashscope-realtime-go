package websocket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	coderws "github.com/coder/websocket"
)

var (
	// ErrClosed indicates the websocket transport is closed.
	ErrClosed = errors.New("dashscope: websocket transport is closed")
	// ErrReconnectDisabled indicates reconnect attempts are disabled by config.
	ErrReconnectDisabled = errors.New("dashscope: reconnect is disabled")
)

// Config controls websocket transport behavior.
type Config struct {
	URL     string
	Headers http.Header

	HandshakeTimeout time.Duration
	ReadTimeout      time.Duration
	ReadLimitBytes   int64
	WriteTimeout     time.Duration

	ReconnectAttempts int
	Backoff           BackoffPolicy
}

// ConnectError is returned when websocket handshake fails.
type ConnectError struct {
	Err        error
	StatusCode int
	Body       string
	Headers    http.Header
}

func (e *ConnectError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("websocket dial failed: status=%d err=%v", e.StatusCode, e.Err)
	}
	return fmt.Sprintf("websocket dial failed: %v", e.Err)
}

// Unwrap returns wrapped dial error.
func (e *ConnectError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Conn is a reconnectable websocket connection wrapper.
type Conn struct {
	cfg Config

	mu      sync.RWMutex
	readMu  sync.Mutex
	writeMu sync.Mutex

	conn   *coderws.Conn
	closed bool
}

// Dial opens websocket connection.
func Dial(ctx context.Context, cfg Config) (*Conn, error) {
	cfg = normalizeConfig(cfg)
	if cfg.URL == "" {
		return nil, &ConnectError{Err: errors.New("empty websocket URL")}
	}

	conn, err := dialOnce(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &Conn{cfg: cfg, conn: conn}, nil
}

// Write sends one text frame.
func (c *Conn) Write(ctx context.Context, payload []byte) error {
	conn, err := c.currentConn()
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	writeCtx, cancel := withTimeout(ctx, c.cfg.WriteTimeout)
	defer cancel()

	if err := conn.Write(writeCtx, coderws.MessageText, payload); err != nil {
		return fmt.Errorf("websocket write: %w", err)
	}
	return nil
}

// Read receives one text/binary frame payload.
func (c *Conn) Read(ctx context.Context) ([]byte, error) {
	conn, err := c.currentConn()
	if err != nil {
		return nil, err
	}

	c.readMu.Lock()
	defer c.readMu.Unlock()

	readCtx, cancel := withTimeout(ctx, c.cfg.ReadTimeout)
	defer cancel()

	_, payload, err := conn.Read(readCtx)
	if err != nil {
		return nil, fmt.Errorf("websocket read: %w", err)
	}

	data := make([]byte, len(payload))
	copy(data, payload)
	return data, nil
}

// Reconnect redials websocket with backoff and swaps active connection.
func (c *Conn) Reconnect(ctx context.Context) error {
	c.mu.RLock()
	closed := c.closed
	attempts := c.cfg.ReconnectAttempts
	c.mu.RUnlock()
	if closed {
		return ErrClosed
	}
	if attempts == 0 {
		return ErrReconnectDisabled
	}

	var next *coderws.Conn
	err := Retry(ctx, c.cfg.ReconnectAttempts, c.cfg.Backoff, func(callCtx context.Context) error {
		conn, dialErr := dialOnce(callCtx, c.cfg)
		if dialErr != nil {
			return dialErr
		}
		next = conn
		return nil
	})
	if err != nil {
		return fmt.Errorf("reconnect failed: %w", err)
	}

	var prev *coderws.Conn
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = next.Close(coderws.StatusNormalClosure, "closing")
		return ErrClosed
	}
	prev = c.conn
	c.conn = next
	c.mu.Unlock()

	if prev != nil {
		_ = prev.Close(coderws.StatusServiceRestart, "reconnected")
	}

	return nil
}

// Close closes websocket transport.
func (c *Conn) Close(reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	if conn == nil {
		return nil
	}

	if reason == "" {
		reason = "client closed"
	}
	return conn.Close(coderws.StatusNormalClosure, reason)
}

// IsRetryable reports whether an error should trigger reconnect.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrClosed) {
		return false
	}
	if errors.Is(err, ErrReconnectDisabled) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var connectErr *ConnectError
	if errors.As(err, &connectErr) {
		if connectErr.StatusCode >= 400 && connectErr.StatusCode < 500 && connectErr.StatusCode != http.StatusTooManyRequests {
			return false
		}
		return true
	}

	status := coderws.CloseStatus(err)
	if status != -1 {
		switch status {
		case coderws.StatusNormalClosure,
			coderws.StatusGoingAway,
			coderws.StatusPolicyViolation,
			coderws.StatusUnsupportedData,
			coderws.StatusInvalidFramePayloadData,
			coderws.StatusMessageTooBig,
			coderws.StatusProtocolError:
			return false
		default:
			return true
		}
	}

	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return false
}

func (c *Conn) currentConn() (*coderws.Conn, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.conn == nil {
		return nil, ErrClosed
	}
	return c.conn, nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.Headers == nil {
		cfg.Headers = make(http.Header)
	}
	if cfg.HandshakeTimeout < 0 {
		cfg.HandshakeTimeout = 0
	}
	if cfg.WriteTimeout < 0 {
		cfg.WriteTimeout = 0
	}
	if cfg.ReadTimeout < 0 {
		cfg.ReadTimeout = 0
	}
	if cfg.ReadLimitBytes <= 0 {
		cfg.ReadLimitBytes = 8 << 20
	}
	if cfg.ReconnectAttempts < 0 {
		cfg.ReconnectAttempts = 0
	}
	cfg.Backoff = cfg.Backoff.normalized()
	return cfg
}

func dialOnce(ctx context.Context, cfg Config) (*coderws.Conn, error) {
	dialCtx, cancel := withTimeout(ctx, cfg.HandshakeTimeout)
	defer cancel()

	conn, resp, err := coderws.Dial(dialCtx, cfg.URL, &coderws.DialOptions{HTTPHeader: cfg.Headers})
	if err != nil {
		ce := &ConnectError{Err: err}
		if resp != nil {
			ce.StatusCode = resp.StatusCode
			ce.Headers = cloneHeader(resp.Header)
			if resp.Body != nil {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
				if readErr == nil {
					ce.Body = string(body)
				}
				_ = resp.Body.Close()
			}
		}
		return nil, ce
	}

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	conn.SetReadLimit(cfg.ReadLimitBytes)
	return conn, nil
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func cloneHeader(header http.Header) http.Header {
	if header == nil {
		return nil
	}
	cloned := make(http.Header, len(header))
	for key, values := range header {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}
