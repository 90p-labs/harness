package domain

import (
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type Codec interface {
	Host() shareddomain.Host
	Capabilities() []shareddomain.EventName
	Decode(hook HookName, raw []byte) (shareddomain.Envelope, error)
	Encode(hook HookName, verdict shareddomain.Verdict) ([]byte, error)
}
