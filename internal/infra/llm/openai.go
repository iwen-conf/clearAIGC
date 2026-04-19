package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type ResponsesClient struct {
	requester *requester
}

func NewResponsesClient(cfg domain.ProviderConfig) *ResponsesClient {
	return &ResponsesClient{requester: newRequester(cfg)}
}

type responsesRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Store bool   `json:"store"`
}

type responsesResponse struct {
	ID         string `json:"id"`
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
	} `json:"output"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (c *ResponsesClient) Complete(ctx context.Context, req domain.LLMRequest) (*domain.ProviderResult, error) {
	body := responsesRequest{
		Model: c.requester.cfg.Model,
		Input: req.Prompt,
		Store: c.requester.cfg.Store,
	}
	var out responsesResponse
	resp, err := c.requester.post(ctx, "/v1/responses", req.RequestID, body, &out)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(out.OutputText)
	if text == "" {
		var parts []string
		for _, item := range out.Output {
			for _, content := range item.Content {
				if content.Type == "output_text" || content.Text != "" {
					parts = append(parts, content.Text)
				}
			}
		}
		text = strings.TrimSpace(strings.Join(parts, "\n"))
	}
	if text == "" {
		return nil, fmt.Errorf("provider %s returned empty output", c.requester.cfg.Name)
	}
	return &domain.ProviderResult{
		Provider:     c.requester.cfg.Name,
		OutputText:   text,
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
		RawRequestID: resp.Header.Get("x-request-id"),
	}, nil
}

type ChatClient struct {
	requester *requester
}

func NewChatClient(cfg domain.ProviderConfig) *ChatClient {
	return &ChatClient{requester: newRequester(cfg)}
}

type chatRequest struct {
	Model    string           `json:"model"`
	Messages []domain.Message `json:"messages"`
}

type chatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (c *ChatClient) Complete(ctx context.Context, req domain.LLMRequest) (*domain.ProviderResult, error) {
	body := chatRequest{
		Model: c.requester.cfg.Model,
		Messages: []domain.Message{
			{Role: "user", Content: req.Prompt},
		},
	}
	var out chatResponse
	resp, err := c.requester.post(ctx, "/v1/chat/completions", req.RequestID, body, &out)
	if err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("provider %s returned empty completion", c.requester.cfg.Name)
	}
	return &domain.ProviderResult{
		Provider:     c.requester.cfg.Name,
		OutputText:   strings.TrimSpace(out.Choices[0].Message.Content),
		InputTokens:  out.Usage.PromptTokens,
		OutputTokens: out.Usage.CompletionTokens,
		RawRequestID: resp.Header.Get("x-request-id"),
	}, nil
}
