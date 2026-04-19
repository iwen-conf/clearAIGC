package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

func TestResponsesClientUsesResponsesAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Client-Request-Id"); got != "req-1" {
			t.Fatalf("unexpected request id header: %s", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["store"] != false {
			t.Fatalf("expected store=false, got %#v", body["store"])
		}
		w.Header().Set("x-request-id", "openai-req-1")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"output_text": "rewritten text",
			"usage": map[string]any{
				"input_tokens":  12,
				"output_tokens": 7,
			},
		})
	}))
	defer server.Close()

	client := NewResponsesClient(domain.ProviderConfig{
		Name:     "responses",
		BaseURL:  server.URL,
		APIKey:   "test",
		Model:    "gpt-test",
		Timeout:  time.Second,
		Store:    false,
		RPMLimit: 1000,
	})

	result, err := client.Complete(context.Background(), domain.LLMRequest{
		RequestID: "req-1",
		Prompt:    "hello",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result.OutputText != "rewritten text" {
		t.Fatalf("unexpected output: %q", result.OutputText)
	}
	if result.RawRequestID != "openai-req-1" {
		t.Fatalf("unexpected raw request id: %s", result.RawRequestID)
	}
}

func TestProviderChainFallsBackToChat(t *testing.T) {
	rewriteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer rewriteServer.Close()

	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{"content": "fallback output"},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 3,
			},
		})
	}))
	defer chatServer.Close()

	chain := NewProviderChain(
		NewResponsesClient(domain.ProviderConfig{
			Name:     "responses",
			BaseURL:  rewriteServer.URL,
			APIKey:   "test",
			Model:    "gpt-test",
			Timeout:  time.Second,
			Store:    false,
			RPMLimit: 1000,
		}),
		NewChatClient(domain.ProviderConfig{
			Name:     "chat",
			BaseURL:  chatServer.URL,
			APIKey:   "test",
			Model:    "gpt-test",
			Timeout:  time.Second,
			RPMLimit: 1000,
		}),
	)

	result, err := chain.Complete(context.Background(), domain.LLMRequest{
		RequestID: "req-2",
		Prompt:    "rewrite",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !strings.Contains(result.OutputText, "fallback output") {
		t.Fatalf("unexpected output: %q", result.OutputText)
	}
	if result.Provider != "chat" {
		t.Fatalf("unexpected provider: %s", result.Provider)
	}
}
