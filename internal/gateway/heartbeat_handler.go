package gateway

import (
	"context"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lckrugel/billy-the-bot/internal/gateway/events"
)

type heartbeatManager struct {
	conn             *websocket.Conn
	interval         time.Duration
	lastSequence     *int
	ticker           *time.Ticker
	heartbeatChan    chan struct{}
	heartbeatAckChan chan struct{}
	missedHeartbeats int
}

func newHeartbeatManager(conn *websocket.Conn, interval time.Duration) *heartbeatManager {
	return &heartbeatManager{
		conn:             conn,
		interval:         interval,
		heartbeatChan:    make(chan struct{}, 1),
		heartbeatAckChan: make(chan struct{}, 1),
	}
}

func (hm *heartbeatManager) start(ctx context.Context) {
	slog.Info("starting heartbeat manager", "interval", hm.interval)

	hm.ticker = time.NewTicker(hm.interval)
	defer hm.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping heartbeat manager")
			return
		case <-hm.ticker.C:
			hm.sendHeartbeat()
		case <-hm.heartbeatChan:
			hm.sendHeartbeat()
		case <-hm.heartbeatAckChan:
			hm.handleHeartbeatAck()
		}
	}
}

func (hm *heartbeatManager) requestImmediate() {
	select {
	case hm.heartbeatChan <- struct{}{}:
	default:
		// Channel is full, heartbeat already pending
	}
}

func (hm *heartbeatManager) notifyAck() {
	select {
	case hm.heartbeatAckChan <- struct{}{}:
	default:
		// Channel is full, ack already pending
	}
}

func (hm *heartbeatManager) updateSequence(seq *int) {
	hm.lastSequence = seq
}

func (hm *heartbeatManager) sendHeartbeat() {
	heartbeatEvent := events.NewHeartbeatEvent(hm.lastSequence)
	if err := events.SendEvent(hm.conn, heartbeatEvent); err != nil {
		slog.Error("failed to send heartbeat", "error", err)
		hm.missedHeartbeats++
		return
	}

	if hm.lastSequence != nil {
		slog.Debug("heartbeat sent", "sequence", *hm.lastSequence)
	} else {
		slog.Debug("heartbeat sent", "sequence", "nil")
	}
}

func (hm *heartbeatManager) handleHeartbeatAck() {
	slog.Debug("received heartbeat ack")
	hm.missedHeartbeats = 0
}
