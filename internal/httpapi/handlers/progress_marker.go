package handlers

import (
	"math"

	"mediainfo/internal/httpapi/transport"
)

type screenshotProgressMarker struct {
	current      int
	total        int
	percent      float64
	detail       string
	stepOrder    int
	percentOrder int
	detailOrder  int
}

type screenshotProgressState struct {
	bootstrapMarker     *screenshotProgressMarker
	subtitleMarker      *screenshotProgressMarker
	prepMarker          *screenshotProgressMarker
	packageMarker       *screenshotProgressMarker
	renderMarker        *screenshotProgressMarker
	captureStarted      int
	captureCompleted    int
	captureTotal        int
	captureStartDetail  string
	captureFinishDetail string
	captureStartOrder   int
	captureFinishOrder  int
	uploadTotal         int
	uploadProcessed     int
	uploadFinished      bool
}

func parseScreenshotProgressState(entries []transport.LogEntry) screenshotProgressState {
	state := screenshotProgressState{}

	for _, entry := range entries {
		_ = entry
	}

	return state
}

func bootstrapProgressPercent(marker *screenshotProgressMarker) float64 {
	if marker == nil {
		return 0
	}
	total := maxInt(marker.total, 1)
	processed := maxInt(marker.current, 0)
	return scaledProgress(processed, total, 15)
}

func markerStepProgress(marker *screenshotProgressMarker, base float64) float64 {
	if marker == nil || marker.total <= 0 {
		return base
	}
	return base + float64(marker.current)/float64(marker.total)*15
}

func markerStageProgress(marker *screenshotProgressMarker, base float64) float64 {
	if marker == nil {
		return base
	}
	return markerStepProgress(marker, base)
}

func subtitleProgressPercent(marker *screenshotProgressMarker) float64 {
	if marker == nil || marker.total <= 0 {
		return 0
	}
	if marker.detail == "" {
		return 2
	}
	return math.Min(15+float64(marker.current)/float64(marker.total)*15, 30)
}