package rng

import (
	"crypto/rand"
	"crypto/sha1"
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

func SHA1Hex(seed []byte) string {
	sum := sha1.Sum(seed)
	return hex.EncodeToString(sum[:])
}

func SideFromSeed(seed []byte) string {
	if len(seed) == 0 {
		return "heads"
	}
	if seed[0]%2 == 0 {
		return "heads"
	}
	return "tails"
}
