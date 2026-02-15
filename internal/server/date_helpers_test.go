package server

import (
	"testing"
	"time"
)

func TestCurrentUTCDateString_Parseable(t *testing.T) {
	got := currentUTCDateString()
	if _, err := time.Parse(asOfLayout, got); err != nil {
		t.Fatalf("unexpected date=%q err=%v", got, err)
	}
}
