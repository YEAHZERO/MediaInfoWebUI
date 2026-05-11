package subtitle

import (
	"context"
	"fmt"
	"math"
	"strings"

	screenshotruntime "mediainfo/internal/screenshot/runtime"
)

func BitmapCandidateKey(t float64) string {
	return fmt.Sprintf("%.3f", math.Round(t*1000)/1000)
}

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

func HelperNeedsFFprobe(raw []screenshotruntime.SubtitleTrack, helper []screenshotruntime.BlurayHelperTrack) bool {
	if len(helper) == 0 {
		return true
	}

	helperByPID := make(map[int]screenshotruntime.BlurayHelperTrack, len(helper))
	for _, track := range helper {
		helperByPID[track.PID] = track
	}

	for index, track := range raw {
		helperMeta := screenshotruntime.BlurayHelperTrack{}
		helperMetaOK := false
		if pid, ok := NormalizeStreamPID(track.StreamID); ok {
			if meta, exists := helperByPID[pid]; exists {
				helperMeta = meta
				helperMetaOK = true
			}
		}
		if !helperMetaOK && len(helper) == len(raw) && index < len(helper) {
			helperMeta = helper[index]
			helperMetaOK = true
		}
		if !helperMetaOK {
			return true
		}
		if NeedsBluraySupplement(helperMeta.Lang, "") {
			return true
		}
	}

	return false
}

func HelperHasPayloadBytes(result screenshotruntime.BlurayHelperResult) bool {
	return result.BitrateScanned || result.BitrateMode == "payload-bytes" || result.BitrateMode == "sampled-payload-bytes"
}

func HelperNeedsPayloadScan(raw []screenshotruntime.SubtitleTrack, helperResult screenshotruntime.BlurayHelperResult, helper []screenshotruntime.BlurayHelperTrack, bluray []screenshotruntime.SubtitleTrack, blurayMode string) bool {
	if HelperHasPayloadBytes(helperResult) || len(helper) == 0 {
		return false
	}

	helperByPID := make(map[int]screenshotruntime.BlurayHelperTrack, len(helper))
	for _, track := range helper {
		helperByPID[track.PID] = track
	}

	langCounts := make(map[string]int, 4)
	for index, track := range raw {
		if BitmapKindFromCodec(track.Codec) != screenshotruntime.BitmapSubtitlePGS {
			continue
		}

		langForPick := track.Language
		titleForPick := track.Title
		helperMetaOK := false

		if pid, ok := NormalizeStreamPID(track.StreamID); ok {
			if meta, exists := helperByPID[pid]; exists {
				helperMetaOK = true
				if strings.TrimSpace(meta.Lang) != "" {
					langForPick = strings.ToLower(strings.TrimSpace(meta.Lang))
				}
			}
		}
		if !helperMetaOK && len(helper) == len(raw) && index < len(helper) {
			helperMetaOK = true
			if strings.TrimSpace(helper[index].Lang) != "" {
				langForPick = strings.ToLower(strings.TrimSpace(helper[index].Lang))
			}
		}
		if !helperMetaOK {
			continue
		}

		if (blurayMode == "ffprobe" || blurayMode == "helper+ffprobe") && index < len(bluray) {
			needsSupplement := blurayMode == "ffprobe" || NeedsBluraySupplement(langForPick, titleForPick)
			if needsSupplement {
				if bluray[index].Language != "" && bluray[index].Language != "unknown" {
					langForPick = bluray[index].Language
				}
				if bluray[index].Title != "" {
					titleForPick = bluray[index].Title
				}
			} else if strings.TrimSpace(titleForPick) == "" && bluray[index].Title != "" {
				titleForPick = bluray[index].Title
			}
		}

		langClass := ClassifyLanguage(strings.TrimSpace(langForPick + " " + titleForPick))
		if langClass == "" {
			continue
		}
		langCounts[langClass]++
		if langCounts[langClass] > 1 {
			return true
		}
	}

	return false
}