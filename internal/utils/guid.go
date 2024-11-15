package utils

import "github.com/google/uuid"

func GenerateSessionID() string {
	return uuid.New().String()
}
