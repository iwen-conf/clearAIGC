package scorer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type ExternalDetector interface {
	Name() string
	Score(ctx context.Context, text string) (float64, error)
}

type detectorRequest struct {
	Model    string           `json:"model"`
	Messages []domain.Message `json:"messages"`
}

type detectorResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type detectorPayload struct {
	Score float64 `json:"score"`
}

type OpenAICompatibleDetector struct {
	requester *requester
}

type detectorCacheEntry struct {
	score     float64
	storedAt  time.Time
	expiresAt time.Time
}

type DetectorCacheOption func(*CachingDetector)

type CachingDetector struct {
	inner      ExternalDetector
	ttl        time.Duration
	maxEntries int
	now        func() time.Time

	mu      sync.Mutex
	entries map[string]detectorCacheEntry
}

type ExternalFallback struct{}

func (ExternalFallback) Name() string {
	return "offline-fallback"
}

func (ExternalFallback) Score(_ context.Context, text string) (float64, error) {
	return domain.EstimateAIRate(text), nil
}

func NewOpenAICompatibleDetector(cfg domain.ProviderConfig) *OpenAICompatibleDetector {
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return nil
	}
	return &OpenAICompatibleDetector{requester: newRequester(cfg)}
}

func NewCachingDetector(inner ExternalDetector, opts ...DetectorCacheOption) ExternalDetector {
	if inner == nil {
		return nil
	}
	cached := &CachingDetector{
		inner:      inner,
		ttl:        24 * time.Hour,
		maxEntries: 512,
		now:        time.Now,
		entries:    map[string]detectorCacheEntry{},
	}
	for _, opt := range opts {
		opt(cached)
	}
	if cached.ttl <= 0 {
		return inner
	}
	if cached.maxEntries <= 0 {
		cached.maxEntries = 512
	}
	return cached
}

func WithDetectorCacheTTL(ttl time.Duration) DetectorCacheOption {
	return func(c *CachingDetector) {
		if ttl > 0 {
			c.ttl = ttl
		}
	}
}

func WithDetectorCacheMaxEntries(maxEntries int) DetectorCacheOption {
	return func(c *CachingDetector) {
		if maxEntries > 0 {
			c.maxEntries = maxEntries
		}
	}
}

func (d *OpenAICompatibleDetector) Name() string {
	if d == nil || d.requester == nil {
		return "offline-fallback"
	}
	name := strings.TrimSpace(d.requester.cfg.Name)
	if name == "" {
		name = "openai-detector"
	}
	model := strings.TrimSpace(d.requester.cfg.Model)
	if model == "" {
		return name
	}
	return fmt.Sprintf("%s:%s", name, model)
}

func (d *OpenAICompatibleDetector) Score(ctx context.Context, text string) (float64, error) {
	if d == nil || d.requester == nil {
		return 0, fmt.Errorf("detector not configured")
	}
	body := detectorRequest{
		Model: d.requester.cfg.Model,
		Messages: []domain.Message{
			{Role: "system", Content: detectorSystemPrompt},
			{Role: "user", Content: text},
		},
	}
	var out detectorResponse
	if err := d.requester.post(ctx, "/v1/chat/completions", body, &out); err != nil {
		return 0, err
	}
	if len(out.Choices) == 0 {
		return 0, fmt.Errorf("detector returned no choices")
	}
	return parseDetectorScore(out.Choices[0].Message.Content)
}

func (d *CachingDetector) Name() string {
	if d == nil || d.inner == nil {
		return "offline-fallback"
	}
	return d.inner.Name()
}

func (d *CachingDetector) Score(ctx context.Context, text string) (float64, error) {
	if d == nil || d.inner == nil {
		return 0, fmt.Errorf("detector not configured")
	}
	if d.ttl <= 0 {
		return d.inner.Score(ctx, text)
	}

	key := detectorCacheKey(text)
	now := d.now().UTC()
	if score, ok := d.lookup(key, now); ok {
		return score, nil
	}

	score, err := d.inner.Score(ctx, text)
	if err != nil {
		return 0, err
	}
	d.store(key, score, now)
	return score, nil
}

const detectorSystemPrompt = `You are an AI-writing risk classifier.
Read the user's text and return JSON only with schema {"score":0.0}.
The score must be a single float from 0.0 to 1.0 where higher means the text sounds more like templated AI-generated writing.
Do not return prose, explanations, markdown, or extra keys.`

var detectorScorePattern = regexp.MustCompile(`([01](?:\.\d+)?)`)

func parseDetectorScore(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("detector returned empty content")
	}
	var payload detectorPayload
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &payload); err == nil {
		return math.Max(0, math.Min(1, payload.Score)), nil
	}
	match := detectorScorePattern.FindStringSubmatch(raw)
	if len(match) != 2 {
		return 0, fmt.Errorf("detector response missing score")
	}
	score, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}
	return math.Max(0, math.Min(1, score)), nil
}

func inferDetectorMode(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return "offline"
	}
	if strings.Contains(name, "fallback") {
		return "offline_fallback"
	}
	return name
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}
	return raw
}

func (d *CachingDetector) lookup(key string, now time.Time) (float64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pruneExpiredLocked(now)
	entry, ok := d.entries[key]
	if !ok || !entry.expiresAt.After(now) {
		return 0, false
	}
	return entry.score, true
}

func (d *CachingDetector) store(key string, score float64, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pruneExpiredLocked(now)
	d.entries[key] = detectorCacheEntry{
		score:     math.Max(0, math.Min(1, score)),
		storedAt:  now,
		expiresAt: now.Add(d.ttl),
	}
	for len(d.entries) > d.maxEntries {
		d.evictOldestLocked()
	}
}

func (d *CachingDetector) pruneExpiredLocked(now time.Time) {
	for key, entry := range d.entries {
		if !entry.expiresAt.After(now) {
			delete(d.entries, key)
		}
	}
}

func (d *CachingDetector) evictOldestLocked() {
	var (
		oldestKey string
		oldestAt  time.Time
		found     bool
	)
	for key, entry := range d.entries {
		if !found || entry.storedAt.Before(oldestAt) {
			oldestKey = key
			oldestAt = entry.storedAt
			found = true
		}
	}
	if found {
		delete(d.entries, oldestKey)
	}
}

func detectorCacheKey(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}
