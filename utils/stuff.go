package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandHexString(n int) string {
	buf := make([]byte, n/2)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
