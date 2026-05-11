package handlers

import "strconv"
import "strings"
import "mediainfo/internal/httpapi/transport"

func finalizeProgress(base *transport.TaskProgress, stage, detail string, indeterminate bool) *transport.TaskProgress {
	if base == nil {
		return progressSnapshot(100, stage, detail, 0, 0, indeterminate)
	}

	return progressSnapshot(
		maxFloat(base.Percent, 1),
		stage,
		detail,
		base.Current,
		base.Total,
		indeterminate,
	)
}

func progressSnapshot(percent float64, stage, detail string, current, total int, indeterminate bool) *transport.TaskProgress {
	progress := &transport.TaskProgress{
		Percent:       clampPercent(percent),
		Stage:         stage,
		Detail:        detail,
		Indeterminate: indeterminate,
	}
	if current > 0 {
		progress.Current = current
	}
	if total > 0 {
		progress.Total = total
	}
	return progress
}

func scaledProgress(current, total, width int) float64 {
	if total <= 0 || width <= 0 {
		return 0
	}
	boundedCurrent := clampInt(current, 0, total)
	return float64(boundedCurrent) / float64(total) * float64(width)
}

func scaledProgressFloat(current float64, total, width int) float64 {
	if total <= 0 || width <= 0 {
		return 0
	}
	if current < 0 {
		current = 0
	}
	maxCurrent := float64(total)
	if current > maxCurrent {
		current = maxCurrent
	}
	return current / float64(total) * float64(width)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampPercent(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

func parseInt(values []string, index int) int {
	if index < 0 || index >= len(values) {
		return 0
	}
	value, err := strconv.Atoi(strings.TrimSpace(values[index]))
	if err != nil {
		return 0
	}
	return value
}

func progressPercent(progress *transport.TaskProgress) float64 {
	if progress == nil {
		return 0
	}
	return progress.Percent
}

func progressCurrent(progress *transport.TaskProgress) int {
	if progress == nil {
		return 0
	}
	return progress.Current
}

func progressTotal(progress *transport.TaskProgress) int {
	if progress == nil {
		return 0
	}
	return progress.Total
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}