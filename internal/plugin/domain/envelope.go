package domain

import (
	"encoding/json"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const EnvelopeVersion = 1

type RunRef struct {
	ID    string `json:"id"`
	Phase string `json:"phase"`
}

type Envelope struct {
	Version int               `json:"version"`
	Event   EventName         `json:"event"`
	Host    shareddomain.Host `json:"host"`
	Cwd     string            `json:"cwd"`
	Run     RunRef            `json:"run"`
	Payload json.RawMessage   `json:"payload"`
}
