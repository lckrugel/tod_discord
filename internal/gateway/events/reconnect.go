package events

import (
	"fmt"
)

type ReconnectEvent struct {
	Event
}

func NewReconnectEvent() ReconnectEvent {
	return ReconnectEvent{
		Event: Event{
			Operation: Reconnect,
		},
	}
}

func (recEvent *ReconnectEvent) DecodeData(e Event) error {
	if e.Operation != Reconnect {
		return fmt.Errorf("unexpected event received: expected Reconnect, got %v", e.Operation.String())
	}

	recEvent.Event = e
	return nil
}
