// Package digest owns the "sha256:<64 lowercase hex>" form every file and
// content digest of the hub takes. It imports nothing from the module, so any
// layer may use it.
package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const prefix = "sha256:"

// Bytes returns the SHA-256 digest of data as sha256:<64 lowercase hex>.
func Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return prefix + hex.EncodeToString(sum[:])
}

// Valid reports whether value is a digest in the form Bytes returns:
// sha256: followed by exactly 64 lowercase hex characters.
func Valid(value string) bool {
	hexPart, ok := strings.CutPrefix(value, prefix)
	if !ok || len(hexPart) != sha256.Size*2 {
		return false
	}
	for _, c := range hexPart {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
