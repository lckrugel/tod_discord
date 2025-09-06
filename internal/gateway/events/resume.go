package events

import (
	"encoding/json"
	"fmt"
)

type ResumeEvent struct {
	Event
	Data ResumePayload `json:"d"`
}

type ResumePayload struct {
	Token     string `json:"token"`
	SessionId string `json:"session_id"`
	Sequence  *int   `json:"seq"`
}

func NewResumeEvent(payload ResumePayload) ResumeEvent {
	return ResumeEvent{
		Event: Event{
			Operation: Resume,
		},
		Data: payload,
	}
}

func (resEvent ResumeEvent) prepareToSend() ([]byte, error) {
	json, err := json.Marshal(resEvent)
	if err != nil {
		return nil, fmt.Errorf("error preparing Resume event: %w", err)
	}
	return json, nil
}
