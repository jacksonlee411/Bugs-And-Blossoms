package server

import (
	"net/http"
	"testing"
)

func TestEffectiveHost_TrustProxy(t *testing.T) {
	t.Setenv("TRUST_PROXY", "1")

	r := &http.Request{Header: http.Header{}, Host: "ignored:8080"}
	r.Header.Set("X-Forwarded-Host", "Example.COM:1234, other")
	if got := effectiveHost(r); got != "example.com" {
		t.Fatalf("got=%q", got)
	}
}

func TestEffectiveHost_NoProxyTrust(t *testing.T) {
	t.Setenv("TRUST_PROXY", "")

	r := &http.Request{Header: http.Header{}, Host: "Example.COM:8080"}
	r.Header.Set("X-Forwarded-Host", "should-not-use.local")
	if got := effectiveHost(r); got != "example.com" {
		t.Fatalf("got=%q", got)
	}
}

func TestForwardedHost_Empty(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if got := forwardedHost(r); got != "" {
		t.Fatalf("got=%q", got)
	}
}
