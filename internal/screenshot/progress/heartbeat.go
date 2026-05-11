package progress

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

func StartHeartbeat(ctx context.Context, onTick func(elapsed time.Duration)) func() {
	if ctx == nil || onTick == nil {
		return func() {}
	}
	startAt := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	done := make(chan struct{}, 1)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				onTick(time.Since(startAt))
			case <-ctx.Done():
				close(done)
				return
			}
		}
	}()

	return func() {
		ticker.Stop()
		select {
		case <-done:
		default:
		}
	}
}

func ClampPercent(percent float64) float64 {
	switch {
	case percent < 0:
		return 0
	case percent > 100:
		return 100
	default:
		return math.Round(percent*10) / 10
	}
}

func SubtitleHeartbeatStepPercent(elapsed time.Duration) float64 {
	elapsedSeconds := elapsed.Seconds()
	if elapsedSeconds <= 0 {
		return 0
	}

	projected := (elapsedSeconds / 120.0) * 100.0
	if projected > 90 {
		return 90
	}
	return ClampPercent(projected)
}

func SubtitleHeartbeatDetail(detail string, elapsed time.Duration) string {
	elapsedStr := formatElapsedCompact(elapsed)
	if strings.TrimSpace(detail) == "" {
		return fmt.Sprintf("正在准备字幕：%s", elapsedStr)
	}
	return fmt.Sprintf("%s：%s", strings.TrimRight(strings.TrimSpace(detail), "。"), elapsedStr)
}

func formatElapsedCompact(elapsed time.Duration) string {
	seconds := int(elapsed.Seconds())
	switch {
	case seconds < 60:
		return fmt.Sprintf("%ds", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	default:
		return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
	}
}