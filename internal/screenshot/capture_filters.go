package screenshot

import (
	"fmt"
	"strings"

	screenshotruntime "mediainfo/internal/screenshot/runtime"
	screenshottimestamps "mediainfo/internal/screenshot/timestamps"
)

func (r *screenshotRunner) displayAspectFilter() string {
	if strings.TrimSpace(r.render.AspectChain) != "" {
		return r.render.AspectChain
	}
	return buildDisplayAspectFilter()
}

func buildDisplayAspectFilter() string {
	return ""
}

func joinFilters(parts ...string) string {
	filters := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filters = append(filters, part)
	}
	return strings.Join(filters, ",")
}

func (r *screenshotRunner) bitmapSubtitleTargetSize() (int, int) {
	if r == nil {
		return 0, 0
	}
	if r.media.DisplayWidth > 0 && r.media.DisplayHeight > 0 {
		return r.media.DisplayWidth, r.media.DisplayHeight
	}
	return r.media.VideoWidth, r.media.VideoHeight
}

func (r *screenshotRunner) hasUsablePGSCanvas() bool {
	if r == nil || !r.isPGSSubtitle() {
		return false
	}
	targetWidth, targetHeight := r.bitmapSubtitleTargetSize()
	return r.render.SubtitleCanvasWidth > 0 && r.render.SubtitleCanvasHeight > 0 && targetWidth > 0 && targetHeight > 0
}

func (r *screenshotRunner) buildPGSSubtitleScaleChain() string {
	if !r.hasUsablePGSCanvas() {
		return ""
	}
	targetWidth, targetHeight := r.bitmapSubtitleTargetSize()
	if targetWidth == r.render.SubtitleCanvasWidth && targetHeight == r.render.SubtitleCanvasHeight {
		return ""
	}
	return fmt.Sprintf("scale=%d:%d", targetWidth, targetHeight)
}

func (r *screenshotRunner) pgsOverlayPosition() string {
	if r.hasUsablePGSCanvas() {
		return "0:0"
	}
	return "(W-w)/2:(H-h-10)"
}

func (r *screenshotRunner) buildPGSOverlayFilterComplex(videoChain, overlayTail string) string {
	steps := []string{
		buildFilterGraphStep("[0:v:0]", videoChain, "[video]"),
		buildFilterGraphStep(fmt.Sprintf("[0:s:%d]", r.subtitle.RelativeIndex), r.buildPGSSubtitleScaleChain(), "[sub]"),
		buildFilterGraphStep("[video][sub]", joinFilters(fmt.Sprintf("overlay=%s", r.pgsOverlayPosition()), overlayTail), "[out]"),
	}
	return strings.Join(steps, ";")
}

func (r *screenshotRunner) buildPGSRenderFilterComplex() string {
	return r.buildPGSOverlayFilterComplex(joinFilters(r.render.ColorChain, r.displayAspectFilter()), "")
}

func buildFilterGraphStep(input, chain, output string) string {
	filterChain := strings.TrimSpace(chain)
	if filterChain == "" {
		filterChain = "null"
	}
	return fmt.Sprintf("%s%s%s", input, filterChain, output)
}

func (r *screenshotRunner) buildTextSubtitleRenderChain(timelineBase, aligned float64, subFilter string) string {
	baseTimeline := fmt.Sprintf("setpts=PTS-STARTPTS+%s/TB", screenshottimestamps.FormatSeconds(timelineBase))
	selectFrame := fmt.Sprintf("select='gte(t,%s)'", screenshottimestamps.FormatSeconds(aligned))
	if r.usesLibplaceboColorspace() {
		return joinFilters(
			baseTimeline,
			selectFrame,
			r.render.ColorChain,
			subFilter,
			r.displayAspectFilter(),
		)
	}
	return joinFilters(
		baseTimeline,
		selectFrame,
		subFilter,
		r.render.ColorChain,
		r.displayAspectFilter(),
	)
}

func (r *screenshotRunner) buildTextSubtitleFilter() string {
	if r.subtitle.Mode == "none" {
		return ""
	}

	sizePart := ""
	if r.media.VideoWidth > 0 && r.media.VideoHeight > 0 {
		sizePart = fmt.Sprintf(":original_size=%dx%d", r.media.VideoWidth, r.media.VideoHeight)
	}
	fontPart := ""
	if strings.TrimSpace(r.subtitleState.SubtitleFontDir) != "" {
		fontPart = fmt.Sprintf(":fontsdir='%s'", escapeFilterValue(r.subtitleState.SubtitleFontDir))
	}

	switch r.subtitle.Mode {
	case "external":
		return fmt.Sprintf("subtitles='%s'%s%s", escapeFilterValue(r.subtitle.File), sizePart, fontPart)
	case "internal":
		return fmt.Sprintf("subtitles='%s'%s%s:si=%d", escapeFilterValue(r.sourcePath), sizePart, fontPart, r.subtitle.RelativeIndex)
	default:
		return ""
	}
}

func (r *screenshotRunner) bitmapSubtitleKind() screenshotruntime.BitmapSubtitleKind {
	return bitmapSubtitleKindFromCodec(r.subtitle.Codec)
}

func bitmapSubtitleKindFromCodec(codec string) screenshotruntime.BitmapSubtitleKind {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "hdmv_pgs_subtitle", "pgssub":
		return screenshotruntime.BitmapSubtitlePGS
	case "dvd_subtitle":
		return screenshotruntime.BitmapSubtitleDVD
	default:
		return screenshotruntime.BitmapSubtitleNone
	}
}

func isUnsupportedBitmapSubtitleCodec(codec string) bool {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "dvb_subtitle", "xsub", "vobsub":
		return true
	default:
		return false
	}
}

func escapeFilterValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	value = strings.ReplaceAll(value, `:`, `\: `)
	value = strings.ReplaceAll(value, `,`, `\,`)
	value = strings.ReplaceAll(value, `;`, `\;`)
	value = strings.ReplaceAll(value, `[`, `\[`)
	value = strings.ReplaceAll(value, `]`, `\]`)
	return value
}

func (r *screenshotRunner) usesLibplaceboColorspace() bool {
	return r.tools.LibplaceboReady && strings.Contains(r.render.ColorChain, "libplacebo=")
}
