package events

import (
	"fmt"
)

type ResumedEvent struct {
	Event
}

func NewResumedEvent() ResumedEvent {
	resumedType := "Resumed"
	return ResumedEvent{
		Event: Event{
			Operation: Dispatch,
			Type:      &resumedType,
		},
	}
}

func (resdEvent *ResumedEvent) DecodeData(e Event) error {
	if e.Operation != Reconnect {
		return fmt.Errorf("unexpected event received: expected Resumed, got %v", e.Operation.String())
	}

	resdEvent.Event = e
	return nil
}
