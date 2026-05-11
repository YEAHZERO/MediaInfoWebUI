package timestamps

import (
	"fmt"
	"strings"
)

func FormatTimestamp(seconds float64) string {
	h := int(seconds / 3600)
	m := int((seconds - float64(h)*3600) / 60)
	s := seconds - float64(h)*3600 - float64(m)*60
	return fmt.Sprintf("%02d:%02d:%06.3f", h, m, s)
}

func FormatSeconds(seconds float64) string {
	return fmt.Sprintf("%.3f", seconds)
}

func SecToHMS(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds / 3600)
	m := int((seconds - float64(h)*3600) / 60)
	s := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func DisplayProbeValue(value string) string {
	lower := strings.ToLower(strings.TrimSpace(value))
	switch lower {
	case "", "unknown", "und", "undefined", "null", "n/a", "na":
		return "无"
	default:
		return strings.TrimSpace(value)
	}
}