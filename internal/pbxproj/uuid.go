package pbxproj

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// NewUUID generates a 24-character uppercase hex string in the Xcode PBX UUID format.
func NewUUID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate PBX UUID: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}
