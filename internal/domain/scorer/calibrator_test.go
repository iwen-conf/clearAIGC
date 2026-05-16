package scorer

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCalibratorPersistsObservedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "detector-state.json")
	calibrator := NewCalibrator(
		WithCalibrationStatePath(path),
		WithCalibrationMinSamples(2),
		WithCalibrationDetector("openai-detector:gpt-4.1-mini"),
	)

	calibrator.Observe(0.30, 0.42, "openai-detector:gpt-4.1-mini")
	calibrator.Observe(0.62, 0.74, "openai-detector:gpt-4.1-mini")

	reloaded := NewCalibrator(WithCalibrationStatePath(path))
	score := reloaded.Apply(0.50)
	if score == nil {
		t.Fatal("expected calibrated score")
	}
	if score.Mode != "online_calibrated" {
		t.Fatalf("expected persisted online mode, got %q", score.Mode)
	}
	if score.SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", score.SampleCount)
	}
	if score.Detector != "openai-detector:gpt-4.1-mini" {
		t.Fatalf("unexpected detector %q", score.Detector)
	}
}

func TestCompositeScoreLocalSkipsExternalFallbackCalibration(t *testing.T) {
	composite := NewComposite()
	score, err := composite.ScoreLocal(context.Background(), "综合来看，这项工作具有现实意义。")
	if err != nil {
		t.Fatalf("ScoreLocal returned error: %v", err)
	}
	if score.Calibrated == nil {
		t.Fatal("expected calibrated score")
	}
	if score.Calibrated.Mode != "local_internal" {
		t.Fatalf("expected local_internal mode, got %q", score.Calibrated.Mode)
	}
}
