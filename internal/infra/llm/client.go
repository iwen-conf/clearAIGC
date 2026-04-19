package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

var ErrAllProvidersUnavailable = errors.New("all providers unavailable")

type ProviderChain struct {
	rewrite  domain.LLMClient
	fallback domain.LLMClient
}

func NewProviderChain(rewrite domain.LLMClient, fallback domain.LLMClient) *ProviderChain {
	return &ProviderChain{rewrite: rewrite, fallback: fallback}
}

func (c *ProviderChain) Complete(ctx context.Context, req domain.LLMRequest) (*domain.ProviderResult, error) {
	result, err := c.rewrite.Complete(ctx, req)
	if err == nil {
		return result, nil
	}

	fallbackResult, fallbackErr := c.fallback.Complete(ctx, req)
	if fallbackErr == nil {
		return fallbackResult, nil
	}

	return nil, fmt.Errorf("%w: rewrite=%v fallback=%v", ErrAllProvidersUnavailable, err, fallbackErr)
}

type requester struct {
	client   *http.Client
	cfg      domain.ProviderConfig
	mu       sync.Mutex
	lastAt   time.Time
	interval time.Duration
}

func newRequester(cfg domain.ProviderConfig) *requester {
	interval := time.Duration(0)
	if cfg.RPMLimit > 0 {
		interval = time.Minute / time.Duration(cfg.RPMLimit)
	}
	return &requester{
		client:   &http.Client{Timeout: cfg.Timeout},
		cfg:      cfg,
		interval: interval,
	}
}

func (r *requester) waitTurn(ctx context.Context) error {
	if r.interval <= 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	wait := r.lastAt.Add(r.interval).Sub(now)
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	r.lastAt = time.Now()
	return nil
}

func (r *requester) post(ctx context.Context, path string, requestID string, body any, out any) (*http.Response, error) {
	if err := r.waitTurn(ctx); err != nil {
		return nil, err
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(r.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Request-Id", requestID)
	if r.cfg.Organization != "" {
		req.Header.Set("OpenAI-Organization", r.cfg.Organization)
	}
	if r.cfg.Project != "" {
		req.Header.Set("OpenAI-Project", r.cfg.Project)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if out == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}()

	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("provider %s returned %d: %s", r.cfg.Name, resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	if out != nil {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return nil, err
		}
	}

	return resp, nil
}
