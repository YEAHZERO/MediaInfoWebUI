package subtitle

import (
	"context"
	"strings"
)

func SelectBestSubtitle(ctx context.Context, ffprobe, sourcePath string) (*SubtitleInfo, error) {
	subs, err := ProbeSubtitles(ctx, ffprobe, sourcePath)
	if err != nil {
		return nil, err
	}
	if len(subs) == 0 {
		return nil, nil
	}

	var best *SubtitleInfo
	bestScore := -1

	for i := range subs {
		sub := &subs[i]
		score := scoreSubtitle(sub)
		if score > bestScore {
			bestScore = score
			best = sub
		}
	}

	return best, nil
}

func scoreSubtitle(sub *SubtitleInfo) int {
	score := 0
	if sub.Forced {
		score += 100
	}
	if sub.Default {
		score += 50
	}
	if strings.EqualFold(sub.Language, "chi") || strings.EqualFold(sub.Language, "zh") {
		score += 30
	}
	if isBitmapSubtitle(sub.CodecName) {
		score += 10
	}
	return score
}

func isBitmapSubtitle(codec string) bool {
	codec = strings.ToLower(codec)
	return strings.Contains(codec, "hdmv_pgs") ||
		strings.Contains(codec, "dvd_subtitle") ||
		strings.Contains(codec, "dvdsub")
}

func GetSubtitleType(codecName string) string {
	codec := strings.ToLower(codecName)
	switch {
	case strings.Contains(codec, "hdmv_pgs"):
		return "pgs"
	case strings.Contains(codec, "dvd_subtitle") || strings.Contains(codec, "dvdsub"):
		return "dvd"
	default:
		return "text"
	}
}

func GetRelativeIndex(ctx context.Context, ffprobe, sourcePath, codecName string) int {
	subs, err := ProbeSubtitles(ctx, ffprobe, sourcePath)
	if err != nil {
		return -1
	}

	targetType := GetSubtitleType(codecName)
	relativeIndex := 0

	for _, sub := range subs {
		if GetSubtitleType(sub.CodecName) == targetType {
			if sub.Index >= 0 {
				return relativeIndex
			}
			relativeIndex++
		}
	}
	return -1
}