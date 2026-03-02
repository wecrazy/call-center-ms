package utils

import (
	"math/rand"
	"time"
)

func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano())) // Ensure randomness
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))] // Randomly pick from charset
	}
	return string(b)
}
