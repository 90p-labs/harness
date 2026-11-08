package domain

import (
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type Codec interface {
	Host() shareddomain.Host
	Capabilities() []shareddomain.EventName
	Decode(hook HookName, raw []byte) (shareddomain.Envelope, error)
	Encode(hook HookName, verdict shareddomain.Verdict) ([]byte, error)
}
