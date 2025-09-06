package events

import (
	"encoding/json"
	"fmt"
)

type IdentifyEvent struct {
	Event
	Data IdentifyPayload `json:"d"`
}

type IdentifyPayload struct {
	Token      string             `json:"token"`
	Properties IdentifyProperties `json:"properties"`
	Intents    uint64             `json:"intents"`
}

type IdentifyProperties struct {
	Os      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

func NewIdentifyEvent(payload IdentifyPayload) *IdentifyEvent {
	return &IdentifyEvent{
		Event: Event{
			Operation: Identify,
			Sequence:  nil,
			Type:      nil,
		},
		Data: payload,
	}
}

func (idEvent IdentifyEvent) prepareToSend() ([]byte, error) {
	b, err := json.Marshal(idEvent)
	if err != nil {
		return nil, fmt.Errorf("error preparing Identify event: %w", err)
	}
	return b, nil
}
