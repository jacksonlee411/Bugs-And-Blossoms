package routing

import "testing"

func TestKnownErrorMessageReplyCodes(t *testing.T) {
	if got := knownErrorMessage("ai_reply_model_target_mismatch"); got == "" {
		t.Fatal("missing ai_reply_model_target_mismatch message")
	}
	if got := knownErrorMessage("ai_reply_render_failed"); got == "" {
		t.Fatal("missing ai_reply_render_failed message")
	}
}
