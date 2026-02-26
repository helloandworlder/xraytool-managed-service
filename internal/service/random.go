package service

import (
	"crypto/rand"
	"fmt"
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

func randomUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		fallback := randomString(32)
		if len(fallback) < 32 {
			fallback += "00000000000000000000000000000000"
		}
		return fmt.Sprintf("%s-%s-%s-%s-%s", fallback[:8], fallback[8:12], fallback[12:16], fallback[16:20], fallback[20:32])
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	)
}
