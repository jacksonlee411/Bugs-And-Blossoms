package server

import (
	"crypto/rand"
	"errors"
	"io"
	"testing"
)

type randErrReader struct{}

func (randErrReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

type seqRandErrReader struct {
	calls int
}

func (r *seqRandErrReader) Read(p []byte) (int, error) {
	if r.calls == 0 {
		r.calls++
		for i := range p {
			p[i] = byte(i + 1)
		}
		return len(p), nil
	}
	return 0, errors.New("boom")
}

func withRandReader(t *testing.T, reader io.Reader, fn func()) {
	t.Helper()
	orig := rand.Reader
	rand.Reader = reader
	defer func() { rand.Reader = orig }()
	fn()
}
