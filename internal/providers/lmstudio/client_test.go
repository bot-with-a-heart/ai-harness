package lmstudio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconfig "ai-harness/internal/config"
	"ai-harness/internal/providers"
)

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"data": []map[string]any{
				{"id": "local-model", "object": "model", "created": 123, "owned_by": "lmstudio"},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/v1")
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(models) != 1 || models[0].ID != "local-model" {
		t.Fatalf("models = %+v", models)
	}
}

func TestAskDiscoversModelAndPostsChatCompletion(t *testing.T) {
	var chatModel string
	var chatPrompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			writeJSON(t, w, map[string]any{
				"data": []map[string]any{
					{"id": "first-local-model"},
				},
			})
		case "/v1/chat/completions":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			var payload struct {
				Model    string `json:"model"`
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			chatModel = payload.Model
			if len(payload.Messages) != 1 {
				t.Fatalf("messages = %+v", payload.Messages)
			}
			chatPrompt = payload.Messages[0].Content
			writeJSON(t, w, map[string]any{
				"model": "first-local-model",
				"choices": []map[string]any{
					{"message": map[string]any{"role": "assistant", "content": "hello from local"}},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/v1")
	response, err := client.Ask(context.Background(), providers.AskRequest{Prompt: "Say hello"})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if chatModel != "first-local-model" {
		t.Fatalf("model = %q", chatModel)
	}
	if chatPrompt != "Say hello" {
		t.Fatalf("prompt = %q", chatPrompt)
	}
	if response.Content != "hello from local" {
		t.Fatalf("content = %q", response.Content)
	}
}

func TestAskReturnsHTTPErrorPreview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad model"))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL+"/v1")
	_, err := client.Ask(context.Background(), providers.AskRequest{Model: "missing", Prompt: "Hello"})
	if err == nil {
		t.Fatal("ask succeeded, want error")
	}
	if !strings.Contains(err.Error(), "400 Bad Request") || !strings.Contains(err.Error(), "bad model") {
		t.Fatalf("error did not include status/body preview: %v", err)
	}
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()

	client, err := New("desktop", appconfig.Provider{
		Type:    "openai-compatible",
		BaseURL: baseURL,
		APIKey:  "test-key",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
