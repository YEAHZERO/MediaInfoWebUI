package handlers

import (
	"net/http"

	"mediainfo/internal/httpapi/transport"
	"mediainfo/internal/screenshot"
)

type screenshotRequest struct {
	Mode         string
	InputPath    string
	Cleanup      func()
	Variant      string
	SubtitleMode string
	Count        int
}

type screenshotRunOptions struct {
	Variant      string
	SubtitleMode string
	Count        int
}

func parseScreenshotFormRequest(r *http.Request) (*screenshotRequest, error) {
	inputPath, cleanup, err := transport.InputPath(r)
	if err != nil {
		return nil, err
	}

	options := normalizeScreenshotFormOptions(r)
	return &screenshotRequest{
		Mode:         screenshot.NormalizeMode(r.FormValue("mode")),
		InputPath:    inputPath,
		Cleanup:      cleanup,
		Variant:      options.Variant,
		SubtitleMode: options.SubtitleMode,
		Count:        options.Count,
	}, nil
}

func normalizeScreenshotFormOptions(r *http.Request) screenshotRunOptions {
	return screenshotRunOptions{
		Variant:      screenshot.NormalizeVariant(r.FormValue("variant")),
		SubtitleMode: screenshot.NormalizeSubtitleMode(r.FormValue("subtitle_mode")),
		Count:        screenshot.NormalizeCount(r.FormValue("count")),
	}
}

func normalizeScreenshotQueryOptions(r *http.Request) screenshotRunOptions {
	return screenshotRunOptions{
		Variant:      screenshot.NormalizeVariant(r.URL.Query().Get("variant")),
		SubtitleMode: screenshot.NormalizeSubtitleMode(r.URL.Query().Get("subtitle_mode")),
		Count:        screenshot.NormalizeCount(r.URL.Query().Get("count")),
	}
}