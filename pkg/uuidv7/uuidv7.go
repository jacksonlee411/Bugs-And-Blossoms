package uuidv7

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/google/uuid"
)

// New returns a UUIDv7 per RFC 9562 (time-ordered, millisecond precision).
func New() (uuid.UUID, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return uuid.Nil, err
	}

	ms := uint64(time.Now().UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// Version 7 (0b0111)
	b[6] = (b[6] & 0x0f) | 0x70
	// Variant RFC 4122 (0b10xxxxxx)
	b[8] = (b[8] & 0x3f) | 0x80

	return uuid.FromBytes(b[:])
}

// NewString returns UUIDv7 string.
func NewString() (string, error) {
	u, err := New()
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
