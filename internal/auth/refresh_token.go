package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// MakeRefreshToken generates a random 256-bit (32-byte) hex-encoded string.
func MakeRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
