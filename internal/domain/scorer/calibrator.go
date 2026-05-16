package scorer

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/iwen-conf/Naturalize/internal/domain"
)

type CalibrationSample struct {
	InternalTotal float64   `json:"internalTotal"`
	DetectorScore float64   `json:"detectorScore"`
	RecordedAt    time.Time `json:"recordedAt"`
}

type CalibrationState struct {
	Detector    string    `json:"detector"`
	Correlation float64   `json:"correlation"`
	Green       float64   `json:"green"`
	Red         float64   `json:"red"`
	Slope       float64   `json:"slope"`
	Offset      float64   `json:"offset"`
	Mode        string    `json:"mode"`
	SampleCount int       `json:"sampleCount"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type persistedCalibration struct {
	State   CalibrationState    `json:"state"`
	Samples []CalibrationSample `json:"samples,omitempty"`
}

type CalibratorOption func(*Calibrator)

type Calibrator struct {
	mu         sync.RWMutex
	statePath  string
	minSamples int
	windowSize int
	state      CalibrationState
	samples    []CalibrationSample
}

func NewCalibrator(opts ...CalibratorOption) *Calibrator {
	c := &Calibrator{
		minSamples: 8,
		windowSize: 128,
		state: CalibrationState{
			Detector:    "internal-calibrated",
			Correlation: 0.62,
			Green:       0.33,
			Red:         0.66,
			Slope:       0.92,
			Offset:      0.04,
			Mode:        "offline_baseline",
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	c.load()
	return c
}

func WithCalibrationStatePath(path string) CalibratorOption {
	return func(c *Calibrator) {
		c.statePath = path
	}
}

func WithCalibrationMinSamples(count int) CalibratorOption {
	return func(c *Calibrator) {
		if count > 0 {
			c.minSamples = count
		}
	}
}

func WithCalibrationWindow(size int) CalibratorOption {
	return func(c *Calibrator) {
		if size > 0 {
			c.windowSize = size
		}
	}
}

func WithCalibrationDetector(name string) CalibratorOption {
	return func(c *Calibrator) {
		name = stringsTrim(name)
		if name != "" && inferDetectorMode(name) != "offline_fallback" && inferDetectorMode(name) != "offline" {
			c.state.Detector = name
		}
	}
}

func (c *Calibrator) Apply(total float64) *domain.CalibratedScore {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	score := clamp(total*c.state.Slope + c.state.Offset)
	return &domain.CalibratedScore{
		Detector:    c.state.Detector,
		Score:       score,
		Correlation: c.state.Correlation,
		Mode:        c.state.Mode,
		SampleCount: c.state.SampleCount,
	}
}

func (c *Calibrator) Thresholds() (float64, float64) {
	if c == nil {
		return 0.33, 0.66
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state.Green, c.state.Red
}

func (c *Calibrator) Mode() string {
	if c == nil {
		return "offline_baseline"
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state.Mode
}

func (c *Calibrator) Observe(internalTotal, detectorScore float64, detectorName string) {
	if c == nil {
		return
	}
	internalTotal = clamp(internalTotal)
	detectorScore = clamp(detectorScore)
	detectorName = stringsTrim(detectorName)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.samples = append(c.samples, CalibrationSample{
		InternalTotal: internalTotal,
		DetectorScore: detectorScore,
		RecordedAt:    time.Now().UTC(),
	})
	if c.windowSize > 0 && len(c.samples) > c.windowSize {
		c.samples = append([]CalibrationSample(nil), c.samples[len(c.samples)-c.windowSize:]...)
	}
	c.state.SampleCount = len(c.samples)
	if detectorName != "" && inferDetectorMode(detectorName) != "offline_fallback" && inferDetectorMode(detectorName) != "offline" {
		c.state.Detector = detectorName
	}
	if len(c.samples) < c.minSamples {
		c.state.Mode = "warming_up"
		c.state.UpdatedAt = time.Now().UTC()
		c.persistLocked()
		return
	}

	slope, offset, correlation := fitLinearCalibration(c.samples)
	c.state.Slope = slope
	c.state.Offset = offset
	c.state.Correlation = correlation
	c.state.Green = quantileCalibration(c.samples, 0.33)
	c.state.Red = quantileCalibration(c.samples, 0.66)
	if c.state.Green >= c.state.Red {
		c.state.Green = 0.33
		c.state.Red = 0.66
	}
	c.state.Mode = "online_calibrated"
	c.state.UpdatedAt = time.Now().UTC()
	c.persistLocked()
}

func (c *Calibrator) WithScoreMode(calibrated *domain.CalibratedScore, mode string) *domain.CalibratedScore {
	if calibrated == nil || c == nil {
		return calibrated
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	cloned := *calibrated
	if stringsTrim(mode) != "" {
		cloned.Mode = mode
	}
	cloned.Detector = c.state.Detector
	cloned.Correlation = c.state.Correlation
	cloned.SampleCount = c.state.SampleCount
	return &cloned
}

func (c *Calibrator) load() {
	if c == nil || stringsTrim(c.statePath) == "" {
		return
	}
	data, err := os.ReadFile(c.statePath)
	if err != nil {
		return
	}
	var persisted persistedCalibration
	if err := json.Unmarshal(data, &persisted); err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if persisted.State.Detector != "" {
		c.state = persisted.State
	}
	if len(persisted.Samples) > 0 {
		c.samples = append([]CalibrationSample(nil), persisted.Samples...)
		c.state.SampleCount = len(c.samples)
	}
}

func (c *Calibrator) persistLocked() {
	if stringsTrim(c.statePath) == "" {
		return
	}
	payload, err := json.MarshalIndent(persistedCalibration{
		State:   c.state,
		Samples: c.samples,
	}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.statePath), 0o755); err != nil {
		return
	}
	tmp := c.statePath + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, c.statePath)
}

func fitLinearCalibration(samples []CalibrationSample) (float64, float64, float64) {
	if len(samples) == 0 {
		return 0.92, 0.04, 0.62
	}
	n := float64(len(samples))
	sumX := 0.0
	sumY := 0.0
	sumXX := 0.0
	sumYY := 0.0
	sumXY := 0.0
	for _, sample := range samples {
		x := clamp(sample.InternalTotal)
		y := clamp(sample.DetectorScore)
		sumX += x
		sumY += y
		sumXX += x * x
		sumYY += y * y
		sumXY += x * y
	}
	denominator := n*sumXX - sumX*sumX
	slope := 0.92
	if math.Abs(denominator) > 1e-9 {
		slope = (n*sumXY - sumX*sumY) / denominator
	}
	if math.IsNaN(slope) || math.IsInf(slope, 0) || slope <= 0 {
		slope = 0.92
	}
	offset := (sumY / n) - slope*(sumX/n)
	if math.IsNaN(offset) || math.IsInf(offset, 0) {
		offset = 0.04
	}

	corrDenominator := math.Sqrt((n*sumXX - sumX*sumX) * (n*sumYY - sumY*sumY))
	correlation := 0.62
	if corrDenominator > 1e-9 {
		correlation = (n*sumXY - sumX*sumY) / corrDenominator
	}
	if math.IsNaN(correlation) || math.IsInf(correlation, 0) {
		correlation = 0.62
	}
	return slope, offset, clamp(math.Abs(correlation))
}

func quantileCalibration(samples []CalibrationSample, q float64) float64 {
	if len(samples) == 0 {
		if q < 0.5 {
			return 0.33
		}
		return 0.66
	}
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, clamp(sample.DetectorScore))
	}
	sort.Float64s(values)
	index := int(math.Round((float64(len(values) - 1)) * q))
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
