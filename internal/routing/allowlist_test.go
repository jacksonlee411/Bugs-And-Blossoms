package routing

import "testing"

func TestParseAllowlistYAML_Errors(t *testing.T) {
	t.Parallel()

	_, err := ParseAllowlistYAML([]byte{0xff})
	if err == nil {
		t.Fatal("expected yaml error")
	}

	_, err = ParseAllowlistYAML([]byte("version: 2\nentrypoints: {}"))
	if err == nil {
		t.Fatal("expected version error")
	}

	_, err = ParseAllowlistYAML([]byte("version: 1"))
	if err == nil {
		t.Fatal("expected entrypoints error")
	}
}
