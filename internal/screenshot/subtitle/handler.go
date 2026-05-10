package subtitle

import "fmt"

type SubtitleHandler interface {
	BuildFilterChain(time float64, subtitleIndex int) string
	BuildOutputArgs(outputDir, filename string) []string
	SubtitleArgs() []string
	NeedsBitmapOverlay() bool
}

type TextSubtitleHandler struct{}

func (h *TextSubtitleHandler) BuildFilterChain(time float64, subtitleIndex int) string {
	if subtitleIndex < 0 {
		return ""
	}
	return fmt.Sprintf("subtitles=input.mkv:si=%d", subtitleIndex)
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