package phaselog

import (
	"log/slog"
	"math"
	"time"
)

// Meta identifies a crawler phase log event. Keep this package independent
// from crawler.RunState so it can be used by both Temporal and crawler-lite.
type Meta struct {
	RunID    int
	Region   string
	Phase    string
	PhaseID  int
	Version  string
	Tier     string
	Division string
	Queue    string
}

func Started(m Meta, attrs ...any) {
	slog.Info("crawler_phase_started", append(base(m), attrs...)...)
}

func Completed(m Meta, attrs ...any) {
	slog.Info("crawler_phase_completed", append(base(m), attrs...)...)
}

func Skipped(m Meta, reason string, attrs ...any) {
	fields := append(base(m), "reason", reason)
	slog.Info("crawler_phase_skipped", append(fields, attrs...)...)
}

func Progress(m Meta, processed, total, failed int, start time.Time, attrs ...any) {
	fields := append(base(m),
		"processed", processed,
		"total", total,
		"pct", pct(processed, total),
		"failed", failed,
	)
	if !start.IsZero() {
		rate := ratePerSecond(processed, start)
		fields = append(fields, "rate_per_s", rate, "eta", eta(processed, total, rate))
	}
	slog.Info("crawler_phase_progress", append(fields, attrs...)...)
}

func Warn(m Meta, event string, attrs ...any) {
	fields := append(base(m), "event", event)
	slog.Warn("crawler_phase_warning", append(fields, attrs...)...)
}

func Step(m Meta, step string, attrs ...any) {
	fields := append(base(m), "step", step)
	slog.Info("crawler_phase_step", append(fields, attrs...)...)
}

func DebugStep(m Meta, step string, attrs ...any) {
	fields := append(base(m), "step", step)
	slog.Debug("crawler_phase_step", append(fields, attrs...)...)
}

func base(m Meta) []any {
	fields := []any{"run_id", m.RunID, "region", m.Region}
	if m.Phase != "" {
		fields = append(fields, "phase", m.Phase)
	}
	if m.PhaseID != 0 {
		fields = append(fields, "phase_id", m.PhaseID)
	}
	if m.Version != "" {
		fields = append(fields, "version", m.Version)
	}
	if m.Tier != "" {
		fields = append(fields, "tier", m.Tier)
	}
	if m.Division != "" {
		fields = append(fields, "division", m.Division)
	}
	if m.Queue != "" {
		fields = append(fields, "queue", m.Queue)
	}
	return fields
}

func pct(processed, total int) int {
	if total <= 0 {
		return 0
	}
	return int(float64(processed) / float64(total) * 100)
}

func ratePerSecond(processed int, start time.Time) float64 {
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return math.Round(float64(processed)/elapsed*10) / 10
}

func eta(processed, total int, rate float64) string {
	remaining := total - processed
	if rate <= 0 || remaining <= 0 {
		return "?"
	}
	return time.Duration(float64(remaining) / rate * float64(time.Second)).Round(time.Second).String()
}
