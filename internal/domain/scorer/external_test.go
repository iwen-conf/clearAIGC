package scorer

import (
	"context"
	"testing"
	"time"
)

type countingDetector struct {
	name  string
	score float64
	calls int
}

func (d *countingDetector) Name() string {
	return d.name
}

func (d *countingDetector) Score(_ context.Context, _ string) (float64, error) {
	d.calls++
	return d.score, nil
}

func TestCachingDetectorCachesByNormalizedTextUntilTTLExpires(t *testing.T) {
	inner := &countingDetector{name: "test-detector", score: 0.42}
	wrapped, ok := NewCachingDetector(inner, WithDetectorCacheTTL(time.Hour)).(*CachingDetector)
	if !ok {
		t.Fatal("expected caching detector wrapper")
	}

	now := time.Date(2026, time.April, 20, 9, 0, 0, 0, time.UTC)
	wrapped.now = func() time.Time { return now }

	first, err := wrapped.Score(context.Background(), "  hello world  ")
	if err != nil {
		t.Fatalf("first score returned error: %v", err)
	}
	second, err := wrapped.Score(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("second score returned error: %v", err)
	}
	if first != second {
		t.Fatalf("expected cached score match, got first=%v second=%v", first, second)
	}
	if inner.calls != 1 {
		t.Fatalf("expected detector to be called once, got %d", inner.calls)
	}

	now = now.Add(2 * time.Hour)
	third, err := wrapped.Score(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("third score returned error: %v", err)
	}
	if third != first {
		t.Fatalf("expected same detector score after refresh, got first=%v third=%v", first, third)
	}
	if inner.calls != 2 {
		t.Fatalf("expected detector to be called twice after TTL expiry, got %d", inner.calls)
	}
}
