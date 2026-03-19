package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	var fakeServer *httptest.Server
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" {
		_ = os.Setenv("OPENAI_API_KEY", "test-key")
	}
	if strings.TrimSpace(os.Getenv("ASSISTANT_MODEL_CONFIG_JSON")) == "" {
		fakeServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.TrimSpace(r.Header.Get("Authorization")) == "" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":{"message":"missing token"}}`))
				return
			}
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/v1/models":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"gpt-5-codex"}]}`))
				return
			case r.Method == http.MethodPost && r.URL.Path == "/v1/chat/completions":
				var req struct {
					Messages []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"messages"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				prompt := ""
				for i := len(req.Messages) - 1; i >= 0; i-- {
					if strings.TrimSpace(req.Messages[i].Role) == "user" {
						prompt = strings.TrimSpace(req.Messages[i].Content)
						break
					}
				}
				payload, _ := json.Marshal(assistantSyntheticSemanticPayloadForPrompt(prompt))
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + strconv.Quote(string(payload)) + `}}]}`))
				return
			default:
				http.NotFound(w, r)
				return
			}
		}))
		assistantOpenAIHTTPClientFactory = func() *http.Client { return fakeServer.Client() }
		payload, err := json.Marshal(defaultAssistantModelConfig())
		if err == nil {
			var cfg assistantModelConfig
			if json.Unmarshal(payload, &cfg) == nil && len(cfg.Providers) > 0 {
				cfg.Providers[0].Endpoint = fakeServer.URL + "/v1"
				if patched, patchErr := json.Marshal(cfg); patchErr == nil {
					payload = patched
				}
			}
			_ = os.Setenv("ASSISTANT_MODEL_CONFIG_JSON", string(payload))
		}
	}
	code := m.Run()
	if fakeServer != nil {
		fakeServer.Close()
	}
	assistantOpenAIHTTPClientFactory = func() *http.Client { return nil }
	os.Exit(code)
}
