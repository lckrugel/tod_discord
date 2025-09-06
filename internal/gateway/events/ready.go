package events

import (
	"encoding/json"
	"fmt"
)

type ReadyEvent struct {
	Event
	Data ReadyPayload
}

type ReadyPayload struct {
	Api_version int    `json:"api_version"`
	Session_id  string `json:"session_id"`
	Resume_url  string `json:"resume_gateway_url"`
	// TODO: User
	// TODO: UnavailableGuilds
	// TODO: Shards
}

func NewReadyEvent() *ReadyEvent {
	typeReady := "Ready"
	return &ReadyEvent{
		Event: Event{
			Operation: Dispatch,
			Type:      &typeReady,
		},
	}
}

func (rEvent *ReadyEvent) DecodeData(e Event) error {
	if e.Operation != Dispatch {
		return fmt.Errorf("unexpected event received: expected Dispatch, got %v", e.Operation.String())
	}

	if *e.Type != "READY" {
		return fmt.Errorf("unexpected event type received: expected READY, got %v", *e.Type)
	}

	var payload ReadyPayload
	err := json.Unmarshal(e.RawData, &payload)
	if err != nil {
		return fmt.Errorf("error decoding Ready event: %v", err)
	}

	rEvent.Event = e
	rEvent.Data = payload
	return nil
}
