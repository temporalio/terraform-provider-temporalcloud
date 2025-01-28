package provider

import (
	"math/rand"
	"time"
)

func randomString(length int) string {
	r := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	const charset = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}
