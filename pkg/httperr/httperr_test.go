package httperr

import "testing"

func TestIsBadRequest(t *testing.T) {
	if IsBadRequest(nil) {
		t.Fatalf("expected false for nil")
	}
	if IsBadRequest(NewBadRequest("bad")) != true {
		t.Fatalf("expected true for BadRequestError")
	}
	if IsBadRequest(assertErr("other")) {
		t.Fatalf("expected false for non-BadRequestError")
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
