package rng

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func NewSeed() ([]byte, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func SHA256Hex(seed []byte) string {
	sum := sha256.Sum256(seed)
	return hex.EncodeToString(sum[:])
}

func SideFromSeed(seed []byte) string {
	sum := sha256.Sum256(seed)
	if sum[0]%2 == 0 {
		return "heads"
	}
	return "tails"
}
