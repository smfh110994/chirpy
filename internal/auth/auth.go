package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

// GetBearerToken extracts the raw JWT from the Authorization header
func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("no authorization header provided")
	}

	// Expect format: "Bearer <token>"
	splitAuth := strings.Split(authHeader, " ")
	if len(splitAuth) < 2 || strings.ToLower(splitAuth[0]) != "bearer" {
		return "", errors.New("malformed authorization header")
	}

	return strings.TrimSpace(splitAuth[1]), nil
}
