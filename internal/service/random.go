package service

import (
	"crypto/rand"
	"math/big"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	max := big.NewInt(int64(len(chars)))
	for i := range out {
		r, err := rand.Int(rand.Reader, max)
		if err != nil {
			out[i] = chars[i%len(chars)]
			continue
		}
		out[i] = chars[r.Int64()]
	}
	return string(out)
}
