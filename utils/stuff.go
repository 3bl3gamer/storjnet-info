package utils

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"

	"github.com/ansel1/merry"
)

func RandHexString(n int) string {
	buf := make([]byte, n/2)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func RequireEnv(key string) (string, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", merry.New("missing required env variable " + key)
	}
	return value, nil
}

func CopyFloat32SliceToBuf(buf []byte, arr []float32) {
	for i, val := range arr {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(val))
	}
}

func CopyInt32SliceToBuf(buf []byte, arr []int32) {
	for i, val := range arr {
		binary.LittleEndian.PutUint32(buf[i*4:], uint32(val))
	}
}
