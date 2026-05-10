package subtitle

import (
	"context"

	"mediainfo/internal/system"
)

func DetectSubtitleType(ctx context.Context, ffprobe, sourcePath string) string {
	best, err := SelectBestSubtitle(ctx, ffprobe, sourcePath)
	if err != nil || best == nil {
		return ""
	}
	return GetSubtitleType(best.CodecName)
}

func DetectSubtitleRelativeIndex(ctx context.Context, ffprobe, sourcePath string) int {
	best, err := SelectBestSubtitle(ctx, ffprobe, sourcePath)
	if err != nil || best == nil {
		return -1
	}
	return GetRelativeIndex(ctx, ffprobe, sourcePath, best.CodecName)
}

func GetSubtitleHandler(ctx context.Context, ffprobe, sourcePath string, subtitleMode string) (SubtitleHandler, int) {
	if subtitleMode == "off" {
		return &NoSubtitleHandler{}, -1
	}

	subtitleType := DetectSubtitleType(ctx, ffprobe, sourcePath)
	index := DetectSubtitleRelativeIndex(ctx, ffprobe, sourcePath)

	switch subtitleType {
	case "pgs":
		return &PGSSubtitleHandler{}, index
	case "dvd":
		return &DVDSubtitleHandler{}, index
	case "text":
		return &TextSubtitleHandler{}, index
	default:
		return &NoSubtitleHandler{}, -1
	}
}