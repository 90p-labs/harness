package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

// Fingerprint returns the first 12 hex digits of a sha256 over the parts.
// A zero byte separates the parts, so ("ab","c") differs from ("a","bc").
func Fingerprint(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}
