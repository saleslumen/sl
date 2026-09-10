package credentials

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/google/uuid"
)

const APIKeyPrefix = "sl_key_"

var (
	ErrNoCredentials = errors.New("no credentials found; run 'sl auth login'")
	ErrInvalidAPIKey = errors.New("api key must look like sl_key_<uuid>_<secret>")
)

func ValidateAPIKey(key string) error {
	if KeyPrefix(key) == "" {
		return ErrInvalidAPIKey
	}
	return nil
}

func KeyPrefix(key string) string {
	if !strings.HasPrefix(key, APIKeyPrefix) {
		return ""
	}
	const uuidLength = 36
	rest := key[len(APIKeyPrefix):]
	if len(rest) <= uuidLength || rest[uuidLength] != '_' {
		return ""
	}
	id := rest[:uuidLength]
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return ""
	}
	secret := rest[uuidLength+1:]
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) == 0 {
		return ""
	}
	return APIKeyPrefix + id
}
