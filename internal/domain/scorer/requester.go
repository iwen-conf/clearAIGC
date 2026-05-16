package scorer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

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
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &requester{
		client:   &http.Client{Timeout: timeout},
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
	wait := r.lastAt.Add(r.interval).Sub(time.Now())
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

func (r *requester) post(ctx context.Context, path string, body any, out any) error {
	if err := r.waitTurn(ctx); err != nil {
		return err
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(r.cfg.BaseURL, "/")+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.Organization != "" {
		req.Header.Set("OpenAI-Organization", r.cfg.Organization)
	}
	if r.cfg.Project != "" {
		req.Header.Set("OpenAI-Project", r.cfg.Project)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s returned %d: %s", r.cfg.Name, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
