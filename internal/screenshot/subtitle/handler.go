package subtitle

import (
	"context"
	"fmt"
)

type SubtitleHandler interface {
	BuildFilterChain(time float64, subtitleIndex int) string
	BuildOutputArgs(outputDir, filename string) []string
	SubtitleArgs() []string
	NeedsBitmapOverlay() bool
}

func GetSubtitleHandler(ctx context.Context, ffprobe, sourcePath, subtitleMode string) (SubtitleHandler, int) {
	mode := NormalizeSubtitleMode(subtitleMode)
	if mode == "none" {
		return &NoSubtitleHandler{}, -1
	}

	subtitleType := DetectSubtitleType(ctx, ffprobe, sourcePath)

	switch subtitleType {
	case "dvd":
		idx := DetectSubtitleRelativeIndex(ctx, ffprobe, sourcePath)
		return &DVDSubtitleHandler{}, idx
	case "pgs":
		idx := DetectSubtitleRelativeIndex(ctx, ffprobe, sourcePath)
		return &PGSSubtitleHandler{}, idx
	case "text":
		idx := DetectSubtitleRelativeIndex(ctx, ffprobe, sourcePath)
		return &TextSubtitleHandler{SourcePath: sourcePath}, idx
	default:
		return &NoSubtitleHandler{}, -1
	}
}

func NormalizeSubtitleMode(mode string) string {
	switch mode {
	case "none", "disable", "disabled", "off", "0", "false":
		return "none"
	case "force", "forced", "1", "true":
		return "force"
	default:
		return "auto"
	}
}

type TextSubtitleHandler struct {
	SourcePath string
}

func (h *TextSubtitleHandler) BuildFilterChain(time float64, subtitleIndex int) string {
	if subtitleIndex < 0 {
		return ""
	}
	return fmt.Sprintf("subtitles=%s:si=%d", h.SourcePath, subtitleIndex)
}

func (h *TextSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{fmt.Sprintf("%s/%s", outputDir, filename)}
}

func (h *TextSubtitleHandler) SubtitleArgs() []string {
	return nil
}

func (h *TextSubtitleHandler) NeedsBitmapOverlay() bool {
	return false
}

type PGSSubtitleHandler struct{}

func (h *PGSSubtitleHandler) BuildFilterChain(time float64, subtitleIndex int) string {
	return ""
}

func (h *PGSSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{fmt.Sprintf("%s/%s", outputDir, filename)}
}

func (h *PGSSubtitleHandler) SubtitleArgs() []string {
	return nil
}

func (h *PGSSubtitleHandler) NeedsBitmapOverlay() bool {
	return true
}

type DVDSubtitleHandler struct{}

func (h *DVDSubtitleHandler) BuildFilterChain(time float64, subtitleIndex int) string {
	return ""
}

func (h *DVDSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{fmt.Sprintf("%s/%s", outputDir, filename)}
}

func (h *DVDSubtitleHandler) SubtitleArgs() []string {
	return nil
}

func (h *DVDSubtitleHandler) NeedsBitmapOverlay() bool {
	return true
}

type NoSubtitleHandler struct{}

func (h *NoSubtitleHandler) BuildFilterChain(time float64, subtitleIndex int) string {
	return ""
}

func (h *NoSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{fmt.Sprintf("%s/%s", outputDir, filename)}
}

func (h *NoSubtitleHandler) SubtitleArgs() []string {
	return nil
}

func (h *NoSubtitleHandler) NeedsBitmapOverlay() bool {
	return false
}