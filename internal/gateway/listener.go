package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lckrugel/billy-the-bot/internal/gateway/events"
)

type eventListener struct {
	conn             *websocket.Conn
	heartbeatManager *heartbeatManager
	reconnectChan    chan struct{}
	resumeChan       chan struct{}
}

func newEventListener(conn *websocket.Conn, heartbeatManager *heartbeatManager) *eventListener {
	return &eventListener{
		conn:             conn,
		heartbeatManager: heartbeatManager,
		reconnectChan:    make(chan struct{}, 1),
		resumeChan:       make(chan struct{}, 1),
	}
}

func (el *eventListener) start(ctx context.Context) {
	slog.Info("starting event listener")

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping event listener")
			return
		default:
			if err := el.readAndHandleEvent(ctx); err != nil {
				if code, text, ok := isGatewayCloseError(err); ok {
					slog.Debug("websocket closed", "code", code, "text", text)
					slog.Info("stopping event listener")
					el.requestResume()
					return
				}
				if isConnectionClosed(err) {
					slog.Debug("connection closed, attempting to resume")
					slog.Info("stopping event listener")
					el.requestResume()
					return
				}
				if isTimeoutError(err) {
					continue // Timeout is expected, continue to check context
				}
				if strings.Contains(err.Error(), "context cancelled") {
					slog.Info("stopping event listener")
					return
				}
				slog.Warn("error handling event", "error", err)
			}
		}
	}

}

func (el *eventListener) readAndHandleEvent(ctx context.Context) error {
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Channel to receive the read result
	type readResult struct {
		msg []byte
		err error
	}
	resultChan := make(chan readResult, 1)

	// Start reading in a separate goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- readResult{
					msg: nil,
					err: fmt.Errorf("websocket read panic: %v", r),
				}
			}
		}()
		_, msg, err := el.conn.ReadMessage()
		resultChan <- readResult{msg: msg, err: err}
	}()

	// Wait for either the read to complete or context cancellation/timeout
	select {
	case <-readCtx.Done():
		if readCtx.Err() == context.DeadlineExceeded {
			return nil
		}
		return fmt.Errorf("context cancelled")
	case result := <-resultChan:
		if result.err != nil {
			return result.err
		}
		return el.handleEvent(result.msg)
	}
}

func (el *eventListener) handleEvent(msg []byte) error {
	event, err := events.NewEvent(msg)
	if err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	if event.Sequence != nil {
		el.heartbeatManager.updateSequence(event.Sequence)
	}

	switch event.Operation {
	case events.Heartbeat:
		slog.Debug("received heartbeat request")
		el.heartbeatManager.requestImmediate()

	case events.Heartbeat_ACK:
		el.heartbeatManager.notifyAck()

	case events.Dispatch:
		if err := el.handleDispatchEvent(event); err != nil {
			return fmt.Errorf("failed to handle dispatch event: %w", err)
		}

	case events.Reconnect:
		slog.Info("received reconnect request")
		el.requestReconnect()

	case events.Invalid_Session:
		var invalidSessionEvent events.InvalidSessionEvent
		err := invalidSessionEvent.DecodeData(*event)
		if err != nil {
			return fmt.Errorf("failed to decode invalid session event")
		}
		if invalidSessionEvent.Resumable {
			slog.Info("invalid session, resuming...")
			el.requestResume()
		} else {
			slog.Info("invalid session, reconnecting...")
			el.requestReconnect()
		}

	case events.Hello:
		slog.Info("received Hello, reconnecting...")
		el.requestReconnect()

	default:
		slog.Debug("received unhandled event", "operation", event.Operation)
	}

	return nil
}

func (el *eventListener) handleDispatchEvent(event *events.Event) error {
	if event.Operation != events.Dispatch {
		return nil
	}
	switch *event.Type {
	case "RESUMED":
		slog.Info("resumed")
	default:
		slog.Debug("unhandled dispatch event", "type", *event.Type)
	}
	return nil
}

func (el *eventListener) requestReconnect() {
	select {
	case el.reconnectChan <- struct{}{}:
	default:
		// Channel is full, reconnect already requested
	}
}

func (el *eventListener) requestResume() {
	select {
	case el.resumeChan <- struct{}{}:
	default:
		// Channel is full, reconnect already requested
	}
}

func isConnectionClosed(err error) bool {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return true
	}
	if websocket.IsUnexpectedCloseError(err) {
		return true
	}
	if netErr, ok := err.(*net.OpError); ok && netErr.Op == "read" {
		return true
	}
	errStr := err.Error()
	connectionClosedMessages := []string{
		"connection reset",
		"broken pipe",
		"use of closed network connection",
		"repeated read on failed websocket connection",
		"websocket: close",
		"EOF",
	}

	for _, msg := range connectionClosedMessages {
		if strings.Contains(errStr, msg) {
			return true
		}
	}

	return false
}

func isTimeoutError(err error) bool {
	if netErr, ok := err.(*net.OpError); ok {
		return netErr.Timeout()
	}
	if websocket.IsCloseError(err, websocket.CloseNoStatusReceived) {
		return true
	}
	if err != nil && err.Error() == "i/o timeout" {
		return true
	}
	if err != nil && err.Error() == "read timeout" {
		return true
	}
	if err != nil && (err.Error() == "i/o timeout" ||
		err.Error() == "read tcp: i/o timeout" ||
		strings.Contains(err.Error(), "i/o timeout")) {
		return true
	}

	return false
}

func isGatewayCloseError(err error) (code int, text string, ok bool) {
	if ce, ok := err.(*websocket.CloseError); ok {
		return ce.Code, ce.Text, true
	}
	return 0, "", false
}
