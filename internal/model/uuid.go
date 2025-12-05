package model

import "github.com/google/uuid"

// generateUUID creates a new UUID string.
func generateUUID() string {
	return uuid.New().String()
}
