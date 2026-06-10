package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
)

const PrefixHexLen = 12

func SumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:PrefixHexLen]
}

func SumString(s string) string {
	return SumBytes([]byte(s))
}

func SumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return short(h), nil
}

func EmptySHA256() string {
	return SumBytes(nil)
}

type Hasher struct {
	h hash.Hash
}

func NewHasher() *Hasher {
	return &Hasher{h: sha256.New()}
}

func (h *Hasher) Write(p []byte) (int, error) {
	return h.h.Write(p)
}

func (h *Hasher) WriteString(s string) {
	_, _ = h.h.Write([]byte(s))
}

func (h *Hasher) Sum() string {
	return short(h.h)
}

func short(h hash.Hash) string {
	return hex.EncodeToString(h.Sum(nil))[:PrefixHexLen]
}
