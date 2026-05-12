package screenshot

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"mediainfo/internal/screenshot/engine"
	screenshotprogress "mediainfo/internal/screenshot/progress"
	screenshotruntime "mediainfo/internal/screenshot/runtime"
	screenshotsubtitle "mediainfo/internal/screenshot/subtitle"
	screenshottimestamps "mediainfo/internal/screenshot/timestamps"
	"mediainfo/internal/screenshot/taskprogress"
)

func (r *screenshotRunner) logs() string {
	if r == nil { return "" }
	return r.logger.Text()
}

func (r *screenshotRunner) logf(format string, args ...interface{}) {
	if r == nil { return }
	r.logger.Addf(format, args...)
}

func (r *screenshotRunner) logProgress(stage string, current, total int, detail string) {
	r.logf("%s", taskprogress.FormatStep(stage, current, total, detail))
}

func (r *screenshotRunner) logProgressPercent(stage string, percent float64, detail string) {
	r.logf("%s", taskprogress.FormatPercent(stage, percent, detail))
}

func (r *screenshotRunner) startProgressHeartbeat(stage, detail string) func() {
	if r == nil || stage == "" || detail == "" { return func() {} }
	return screenshotprogress.StartHeartbeat(r.ctx, func(elapsed time.Duration) {
		r.logProgressPercent(stage, screenshotprogress.SubtitleHeartbeatStepPercent(elapsed), screenshotprogress.SubtitleHeartbeatDetail(detail, elapsed))
	})
}

func (r *screenshotRunner) lossyPNGFileList() []string {
	if r == nil || len(r.lossyPNGFiles) == 0 { return nil }
	files := make([]string, 0, len(r.lossyPNGFiles))
	for f := range r.lossyPNGFiles { files = append(files, f) }
	return files
}

func (r *screenshotRunner) logScreenshotRunSummary(state *screenshotRunState) {
	r.logf("")
	r.logf("===== 任务完成 =====")
	r.logf("成功: %d 张 | 失败: %d 张", state.successCount, len(state.failures))
	if len(state.failures) == 0 { return }
	r.logf("")
	r.logf("===== 失败详情 =====")
	for _, item := range state.failures { r.logf("%s", item) }
}

func (r *screenshotRunner) logShotAlignmentProgress() {
	if !r.activeShot.Active() { return }
	r.logProgressPercent("截图对齐", 1, r.activeShot.AlignmentDetail())
}

func (r *screenshotRunner) logBitmapSubtitleVisibilityProgress() {
	if !r.activeShot.Active() { return }
	r.logProgressPercent("截图对齐", 50, r.activeShot.BitmapVisibilityDetail(""))
}

func (r *screenshotRunner) logColorspacePlan() {
	if r.render.ColorInfo == "" {
		r.logf("[信息] 无法检测色彩空间，将使用标准转换")
		return
	}
	r.logf("[信息] 检测到色彩空间：%s", strings.TrimSuffix(r.render.ColorInfo, "|"))
	if r.render.ColorChain == "" { return }
	if r.tools.LibplaceboReady && strings.Contains(r.render.ColorChain, "libplacebo=") {
		r.logf("[信息] HDR/WCG 主截图将统一应用 libplacebo tone mapping / 色域映射。")
		return
	}
	r.logf("[信息] HDR/WCG 主截图将统一应用 tone mapping / 色域映射。")
}

func (r *screenshotRunner) runFFmpegSubtitleExtract(args []string) (string, string, error) {
	return "", "", nil
}

func (r *screenshotRunner) isSupportedBitmapSubtitle() bool {
	return r.subtitle.Mode != "none" && (r.subtitle.Codec == "hdmv_pgs_subtitle" || r.subtitle.Codec == "dvd_subtitle")
}

func (r *screenshotRunner) isPGSSubtitle() bool {
	return r.subtitle.Codec == "hdmv_pgs_subtitle"
}

func (r *screenshotRunner) isDVDSubtitle() bool {
	return r.subtitle.Codec == "dvd_subtitle"
}

const subtitleSnapEpsilon = 0.50

func (r *screenshotRunner) alignToSubtitle(requested float64) float64 {
	if r.subtitle.Mode == "none" {
		return requested
	}

	index := r.ensureSubtitleIndex()
	if len(index) == 0 {
		r.logf("[提示] 全片字幕索引未找到可用字幕事件，按原时间点截图：%s", screenshottimestamps.SecToHMSMS(requested))
		return requested
	}

	if r.subtitle.Mode == "internal" && r.isSupportedBitmapSubtitle() {
		r.logBitmapSubtitleVisibilityProgress()
		if candidate, ok := r.findNearestVisibleBitmapIndexedCandidate(requested); ok {
			return r.logAlignedSubtitleIndexCandidate(requested, candidate)
		}
		r.logf("[提示] 全片字幕索引未找到可见字幕事件，按原时间点截图：%s", screenshottimestamps.SecToHMSMS(requested))
		return requested
	}

	if candidate, ok := screenshotsubtitle.SnapFromIndex(requested, index, subtitleSnapEpsilon); ok {
		candidate = r.clampToDuration(candidate)
		return r.logAlignedSubtitleIndexCandidate(requested, candidate)
	}

	r.logf("[提示] 全片字幕索引未找到可用字幕事件，按原时间点截图：%s", screenshottimestamps.SecToHMSMS(requested))
	return requested
}

func (r *screenshotRunner) logAlignedSubtitleIndexCandidate(requested, candidate float64) float64 {
	candidate = r.clampToDuration(candidate)
	if floatDiffGT(candidate, requested) {
		r.logf("[对齐] 请求 %s → 全片字幕索引 %s", screenshottimestamps.SecToHMSMS(requested), screenshottimestamps.SecToHMSMS(candidate))
	} else {
		r.logf("[提示] 已直接复用全片字幕索引对齐到原时间点附近：%s", screenshottimestamps.SecToHMSMS(candidate))
	}
	return candidate
}

func (r *screenshotRunner) clampToDuration(value float64) float64 {
	if value < 0 {
		return 0
	}
	if r.media.Duration > 0 && value > r.media.Duration {
		return r.media.Duration
	}
	return value
}

func floatDiffGT(a, b float64) bool {
	return math.Abs(a-b) > 0.0005
}

func (r *screenshotRunner) acceptBitmapSubtitleCandidate(label string, candidate float64) (float64, bool) {
	candidate = r.clampToDuration(candidate)
	key := screenshotsubtitle.BitmapCandidateKey(candidate)
	if _, rejected := r.subtitleState.RejectedBitmapCandidates[key]; rejected {
		return 0, false
	}

	visible, err := r.bitmapSubtitleVisibleAt(candidate)
	if err != nil {
		r.logf("[提示] %s候选可视性验证失败，沿用该时间点：%s | 原因：%s",
			label,
			screenshottimestamps.SecToHMSMS(candidate),
			err.Error(),
		)
		return candidate, true
	}
	if !visible {
		if r != nil && r.subtitle.Mode == "internal" && r.isSupportedBitmapSubtitle() {
			shortBack := r.renderCoarseBack()
			longBack := r.settings.CoarseBackPGS
			if longBack > shortBack {
				longVisible, longErr := r.internalBitmapSubtitleVisibleAtWithCoarseBack(candidate, longBack)
				if longErr == nil && longVisible {
					r.subtitleState.BitmapRenderBackOverride = longBack
					r.logf("[提示] %s候选仅在较大回溯窗口下渲染出字幕，后续位图截图改用 %ds 回溯窗口：%s",
						label,
						longBack,
						screenshottimestamps.SecToHMSMS(candidate),
					)
					return candidate, true
				}
			}
		}
		if r.subtitleState.RejectedBitmapCandidates == nil {
			r.subtitleState.RejectedBitmapCandidates = make(map[string]struct{})
		}
		r.subtitleState.RejectedBitmapCandidates[key] = struct{}{}
		r.logf("[提示] %s候选未实际渲染出字幕，继续搜索：%s",
			label,
			screenshottimestamps.SecToHMSMS(candidate),
		)
		return 0, false
	}
	return candidate, true
}

func (r *screenshotRunner) findNearestVisibleBitmapIndexedCandidate(requested float64) (float64, bool) {
	if len(r.ensureSubtitleIndex()) == 0 {
		return 0, false
	}

	spans := append([]screenshotruntime.SubtitleSpan(nil), r.subtitleState.Index...)
	sort.Slice(spans, func(i, j int) bool {
		left := math.Abs(screenshotsubtitle.BitmapSnapPoint(spans[i], subtitleSnapEpsilon) - requested)
		right := math.Abs(screenshotsubtitle.BitmapSnapPoint(spans[j], subtitleSnapEpsilon) - requested)
		if left == right {
			return spans[i].Start < spans[j].Start
		}
		return left < right
	})

	limit := len(spans)
	if limit > 8 {
		limit = 8
	}
	for _, span := range spans[:limit] {
		candidate, ok := r.acceptBitmapSubtitleCandidate("全片字幕索引", screenshotsubtitle.BitmapSnapPoint(span, subtitleSnapEpsilon))
		if ok {
			return candidate, true
		}
	}
	return 0, false
}

func (r *screenshotRunner) resolveUniqueScreenshotSecond(requested, aligned float64, usedSeconds map[int]struct{}) (float64, bool, bool) {
	aligned = r.clampToDuration(aligned)
	second := screenshottimestamps.ScreenshotSecond(aligned)
	if _, exists := usedSeconds[second]; !exists {
		return aligned, false, true
	}

	if r.subtitle.Mode != "none" {
		r.ensureSubtitleIndex()
		for _, candidate := range r.uniqueAlignedCandidatesFromSubtitleIndex(requested) {
			candidate = r.clampToDuration(candidate)
			if _, exists := usedSeconds[screenshottimestamps.ScreenshotSecond(candidate)]; exists {
				continue
			}
			return candidate, true, true
		}
	}

	return 0, false, false
}

func (r *screenshotRunner) uniqueAlignedCandidatesFromSubtitleIndex(requested float64) []float64 {
	if len(r.ensureSubtitleIndex()) == 0 {
		return nil
	}

	type secondCandidate struct {
		value    float64
		distance float64
		second   int
	}

	candidates := make([]secondCandidate, 0, len(r.subtitleState.Index))
	seen := make(map[int]struct{}, len(r.subtitleState.Index))
	for _, span := range r.subtitleState.Index {
		startSecond := screenshottimestamps.ScreenshotSecond(span.Start)
		endSecond := screenshottimestamps.ScreenshotSecond(span.End)
		for second := startSecond; second <= endSecond; second++ {
			secondStart := math.Max(span.Start, float64(second))
			secondEnd := math.Min(span.End, float64(second)+0.999)
			if secondEnd < secondStart {
				continue
			}
			candidate := secondStart + (secondEnd-secondStart)/2
			secondKey := screenshottimestamps.ScreenshotSecond(candidate)
			if _, exists := seen[secondKey]; exists {
				continue
			}
			seen[secondKey] = struct{}{}
			candidates = append(candidates, secondCandidate{
				value:    candidate,
				distance: math.Abs(candidate - requested),
				second:   secondKey,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance == candidates[j].distance {
			if candidates[i].second == candidates[j].second {
				return candidates[i].value < candidates[j].value
			}
			return candidates[i].second < candidates[j].second
		}
		return candidates[i].distance < candidates[j].distance
	})

	values := make([]float64, 0, len(candidates))
	for _, candidate := range candidates {
		values = append(values, candidate.value)
	}
	return values
}

func (r *screenshotRunner) captureScreenshot(aligned float64, outputPath string) error {
	sourcePath := r.sourcePath
	if r.subtitleState.DVDResult != nil && r.subtitleState.DVDResult.SelectedVOBPath != "" {
		sourcePath = r.subtitleState.DVDResult.SelectedVOBPath
		r.logf("[调试] DVD 截图使用 VOB 路径: %s", sourcePath)
	}
	_, err := defaultEngine.Capture(r.ctx, engine.CaptureOptions{
		SourcePath:   sourcePath,
		OutputDir:    outputPath,
		Variant:      r.variant,
		SubtitleMode: r.subtitleMode,
		Count:        1,
	})
	return err
}

func generateScreenshotTimestampsFromSource(ctx context.Context, sourcePath string, count int) []float64 {
	result := make([]float64, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, float64(i*10+5))
	}
	return result
}

func init() {
	_ = fmt.Sprintf
	_ = strings.TrimSpace
}

func (r *screenshotRunner) cleanupTemporarySubtitleResources() {
	if r == nil {
		return
	}
	if r.subtitleState.TempSubtitleFile != "" {
		os.Remove(r.subtitleState.TempSubtitleFile)
	}
	if r.subtitleState.SubtitleFontDir != "" {
		os.RemoveAll(r.subtitleState.SubtitleFontDir)
	}
}