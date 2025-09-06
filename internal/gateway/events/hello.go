package events

import (
	"encoding/json"
	"fmt"
)

type HelloEvent struct {
	Event
	Heartbeat_Interval float64
}

func NewHelloEvent() *HelloEvent {
	return &HelloEvent{
		Event: Event{
			Operation: Hello,
		},
	}
}

func (hEvent *HelloEvent) DecodeData(e Event) error {
	if e.Operation != Hello {
		return fmt.Errorf("unexpected event received: expected Hello, got %v", e.Operation.String())
	}
	var payload struct {
		Heartbeat_Interval float64 `json:"heartbeat_interval"`
	}
	err := json.Unmarshal(e.RawData, &payload)
	if err != nil {
		return fmt.Errorf("error decoding Hello event: %w", err)
	}

	hEvent.Event = e
	hEvent.Heartbeat_Interval = payload.Heartbeat_Interval
	return nil
}
