package taskprogress

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func FormatStep(stage string, current, total int, detail string) string {
	return fmt.Sprintf("%s %s %d/%d: %s", Prefix, strings.TrimSpace(stage), current, total, strings.TrimSpace(detail))
}

func FormatPercent(stage string, percent float64, detail string) string {
	return fmt.Sprintf("%s %s %s%%: %s", Prefix, strings.TrimSpace(stage), formatPercent(percent), strings.TrimSpace(detail))
}

func clampPercent(percent float64) float64 {
	switch {
	case percent < 0:
		return 0
	case percent > 100:
		return 100
	default:
		return math.Round(percent*10) / 10
	}
}

func formatPercent(percent float64) string {
	clamped := clampPercent(percent)
	if math.Abs(clamped-math.Round(clamped)) < 0.05 {
		return strconv.Itoa(int(math.Round(clamped)))
	}
	return fmt.Sprintf("%.1f", clamped)
}