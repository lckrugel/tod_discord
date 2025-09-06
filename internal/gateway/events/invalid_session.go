package events

import (
	"encoding/json"
	"fmt"
)

type InvalidSessionEvent struct {
	Event
	Resumable bool
}

func NewInvalidSessionEvent(resumable bool) InvalidSessionEvent {
	return InvalidSessionEvent{
		Event: Event{
			Operation: Invalid_Session,
		},
	}
}

func (iSE *InvalidSessionEvent) DecodeData(e Event) error {
	if e.Operation != Invalid_Session {
		return fmt.Errorf("unexpected event received: expected Invalid_Session, got %v", e.Operation.String())
	}

	var payload struct {
		Resumable bool `json:"d"`
	}
	err := json.Unmarshal(e.RawData, &payload)
	if err != nil {
		return fmt.Errorf("error decoding InvalidSession event: %w", err)
	}

	iSE.Event = e
	iSE.Resumable = payload.Resumable
	return nil
}
