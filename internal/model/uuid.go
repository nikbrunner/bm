package model

import "github.com/google/uuid"

// GenerateUUID creates a new UUID string.
func GenerateUUID() string {
	return uuid.New().String()
}
