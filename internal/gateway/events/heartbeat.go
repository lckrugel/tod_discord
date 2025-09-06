package events

import (
	"encoding/json"
	"fmt"
)

type HeartbeatEvent struct {
	Event
	LastSequence *int
}

type HeartbeatAckEvent struct {
	Event
}

func NewHeartbeatEvent(sequence *int) *HeartbeatEvent {
	return &HeartbeatEvent{
		Event: Event{
			Operation: Heartbeat,
		},
		LastSequence: sequence,
	}
}

func (hbEvent HeartbeatEvent) prepareToSend() ([]byte, error) {
	msg, err := json.Marshal(hbEvent)
	if err != nil {
		return []byte{}, fmt.Errorf("error preparing Heartbeat event: %w", err)
	}
	return msg, nil
}

func (hbEvent *HeartbeatEvent) DecodeData(e Event) error {
	if e.Operation != Heartbeat {
		return fmt.Errorf("unexpected event received: expected Heartbeat, got %v", e.Operation.String())
	}

	var payload struct {
		LastSequence float64 `json:"d"`
	}
	err := json.Unmarshal(e.RawData, &payload)
	if err != nil {
		return fmt.Errorf("error decoding HeartbeatEvent: %w", err)
	}

	sqcInt := int(payload.LastSequence)

	hbEvent.Event = e
	hbEvent.LastSequence = &sqcInt
	return nil
}

func NewHeartbeatAckEvent() *HeartbeatAckEvent {
	return &HeartbeatAckEvent{
		Event: Event{
			Operation: Heartbeat_ACK,
		},
	}
}

func (hbAckEvent *HeartbeatAckEvent) DecodeData(e Event) error {
	if e.Operation != Heartbeat_ACK {
		return fmt.Errorf("unexpected event received: expected Heartbeat_ACK, got %v", e.Operation.String())
	}

	hbAckEvent.Event = e
	return nil
}
