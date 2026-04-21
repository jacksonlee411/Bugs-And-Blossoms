package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox"
)

func TestCubeBoxCreateConversationAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/conversations", strings.NewReader(`{}`))

	handleCubeBoxCreateConversationAPI(rec, req, cubebox.NewRuntime())

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"conversation"`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}

func TestCubeBoxStreamTurnAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns:stream", strings.NewReader(`{"conversation_id":"conv_1","prompt":"hello","next_sequence":1}`))

	handleCubeBoxStreamTurnAPI(rec, req, cubebox.NewRuntime())

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"type":"turn.agent_message.delta"`) {
		t.Fatalf("missing delta event: %s", body)
	}
	if !strings.Contains(body, `"type":"turn.completed"`) {
		t.Fatalf("missing completed event: %s", body)
	}
}

func TestCubeBoxInterruptTurnAPI(t *testing.T) {
	runtime := cubebox.NewRuntime()
	turn := runtime.StartTurn("conv_1", "hello")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/cubebox/turns/"+turn.TurnID+":interrupt", strings.NewReader(`{"reason":"user_requested"}`))

	handleCubeBoxInterruptTurnAPI(rec, req, runtime)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"interrupted":true`) {
		t.Fatalf("unexpected body=%s", rec.Body.String())
	}
}
