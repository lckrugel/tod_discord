package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lckrugel/billy-the-bot/internal/config"
	"github.com/lckrugel/billy-the-bot/internal/gateway/events"
)

type reconnectType int

const (
	reconnectTypeShutdown reconnectType = iota
	reconnectTypeResume
	reconnectTypeReconnect
)

type GatewayClient struct {
	botCfg            *config.BotConfig
	conn              *websocket.Conn
	heartbeatInterval time.Duration
	lastSequence      *int
	sessionId         string
	resumeUrl         string
	shouldResume      bool
}

func NewGatewayClient(botCfg *config.BotConfig) *GatewayClient {
	return &GatewayClient{
		botCfg:       botCfg,
		shouldResume: false,
	}
}

func (gc *GatewayClient) Run(ctx context.Context) error {
	slog.Info("starting gateway client")

	for {
		select {
		case <-ctx.Done():
			slog.Debug("shutdown requested, disconnecting...")
			return gc.disconnect()
		default:
			if err := gc.establishConnection(); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				slog.Warn("connection failed, retrying in 5 seconds", "error", err)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(5 * time.Second):
					continue
				}
			}

			reconnectType := gc.runWorkers(ctx)

			switch reconnectType {
			case reconnectTypeShutdown:
				return gc.disconnect()
			case reconnectTypeResume:
				slog.Debug("attempting to resume connection")
				gc.shouldResume = true
				continue
			case reconnectTypeReconnect:
				slog.Debug("will reconnect with new identify")
				gc.shouldResume = false
				gc.sessionId = ""
				continue
			}
		}
	}
}

func (gc *GatewayClient) establishConnection() error {
	if gc.shouldResume && gc.sessionId != "" && gc.resumeUrl != "" {
		return gc.resume()
	} else {
		return gc.connect()
	}
}

func (gc *GatewayClient) connect() error {
	slog.Info("starting connection")
	// Get the discord URL
	wssURL, err := getWebsocketURL(gc.botCfg.GetSecretKey())
	if err != nil {
		return fmt.Errorf("error getting websocket url: %w", err)
	}
	slog.Debug("got discord gateway url")

	// Connect to the gateway
	conn, resp, err := websocket.DefaultDialer.Dial(wssURL, nil)
	if err != nil {
		return fmt.Errorf("error establishing connection to gateway: %w", err)
	}
	slog.Debug("connection established to discord gateway")

	// Check if the connection was successfully upgraded to WSS
	if resp.StatusCode != 101 {
		return fmt.Errorf("failed to switch protocols with status: %v", resp.StatusCode)
	}
	gc.conn = conn
	slog.Debug("connection upgraded to wss")

	_, msg, err := gc.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read hello message: %w", err)
	}

	event, err := events.NewEvent(msg)
	if err != nil {
		return fmt.Errorf("failed to parse hello event: %w", err)
	}

	if event.Operation != events.Hello {
		return fmt.Errorf("unexpected event received: got: %s expected: Hello", event.Operation)
	}

	helloEvent := events.NewHelloEvent()
	if err := helloEvent.DecodeData(*event); err != nil {
		return fmt.Errorf("failed to decode hello event: %w", err)
	}
	slog.Debug("hello event received")

	gc.heartbeatInterval = time.Duration(helloEvent.Heartbeat_Interval) * time.Millisecond

	identifyPayload := events.IdentifyPayload{
		Token: gc.botCfg.GetSecretKey(),
		Properties: events.IdentifyProperties{
			Os:      runtime.GOOS,
			Browser: gc.botCfg.BotName,
			Device:  gc.botCfg.BotName,
		},
		Intents: gc.botCfg.GetIntents(),
	}

	identifyEvent := events.NewIdentifyEvent(identifyPayload)
	if err := events.SendEvent(gc.conn, identifyEvent); err != nil {
		return fmt.Errorf("failed to send identify event: %w", err)
	}
	slog.Debug("identify event sent")

	_, msg, err = gc.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read ready message: %w", err)
	}

	event, err = events.NewEvent(msg)
	if err != nil {
		return fmt.Errorf("failed to parse ready event: %w", err)
	}

	if event.Operation != events.Dispatch || *event.Type != "READY" {
		return fmt.Errorf("unexpected event received: got op: %s expected op: Dispatch | got type: %s expected op: READY", event.Operation, *event.Type)
	}

	readyEvent := events.NewReadyEvent()
	if err := readyEvent.DecodeData(*event); err != nil {
		return fmt.Errorf("failed to decode ready event: %w", err)
	}
	slog.Debug("ready event received")

	gc.sessionId = readyEvent.Data.Session_id
	gc.resumeUrl = readyEvent.Data.Resume_url

	slog.Info("connected")
	return nil
}

func (gc *GatewayClient) resume() error {
	slog.Info("resuming connection")
	conn, resp, err := websocket.DefaultDialer.Dial(gc.resumeUrl, nil)
	if err != nil {
		return fmt.Errorf("error re-establishing connection to gateway: %w", err)
	}
	slog.Debug("connection re-established to discord gateway")
	// Check if the connection was successfully upgraded to WSS
	if resp.StatusCode != 101 {
		return fmt.Errorf("failed to switch protocols with status: %v", resp.StatusCode)
	}
	gc.conn = conn
	slog.Debug("connection upgraded to wss")
	gc.conn = conn

	resumePayload := events.ResumePayload{
		Token:     gc.botCfg.GetSecretKey(),
		SessionId: gc.sessionId,
		Sequence:  gc.lastSequence,
	}

	resumeEvent := events.NewResumeEvent(resumePayload)
	if err := events.SendEvent(gc.conn, resumeEvent); err != nil {
		return fmt.Errorf("failed to send resume event: %w", err)
	}
	slog.Debug("resume event sent")

	return nil
}

func (gc *GatewayClient) disconnect() error {
	slog.Info("disconnecting from gateway")

	if gc.conn != nil {
		err := gc.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			slog.Warn("failed to send close message", "error", err)
		}

		if err := gc.conn.Close(); err != nil {
			return fmt.Errorf("failed to close connection: %w", err)
		}
	}

	return nil
}

func (gc *GatewayClient) runWorkers(ctx context.Context) reconnectType {
	slog.Debug("starting workers")

	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	heartbeatManager := newHeartbeatManager(gc.conn, gc.heartbeatInterval)
	eventListener := newEventListener(gc.conn, heartbeatManager)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		heartbeatManager.start(connCtx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		eventListener.start(connCtx)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		slog.Debug("shutdown requested")
		connCancel()
		<-done
		return reconnectTypeShutdown

	case <-eventListener.reconnectChan:
		slog.Debug("reconnection requested")
		connCancel()
		<-done
		return reconnectTypeReconnect

	case <-eventListener.resumeChan:
		slog.Debug("resume requested")
		connCancel()
		<-done
		if gc.sessionId != "" && gc.resumeUrl != "" {
			return reconnectTypeResume
		} else {
			return reconnectTypeReconnect
		}

	case <-done:
		slog.Debug("all workers stopped unexpectedly")
		return reconnectTypeReconnect
	}
}

/* Get the websocket URL from the Discord API */
func getWebsocketURL(api_key string) (string, error) {
	req, err := http.NewRequest("GET", "https://discord.com/api/v9/gateway/bot", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bot "+api_key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", errors.New(string(bodyBytes))
	}

	var bodyMap map[string]any
	err = json.Unmarshal(bodyBytes, &bodyMap)
	if err != nil {
		return "", err
	}

	url, ok := bodyMap["url"].(string)
	if !ok {
		return "", errors.New("invalid url")
	}
	return url, nil
}
