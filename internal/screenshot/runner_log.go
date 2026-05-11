package screenshot

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"mediainfo/internal/screenshot/engine"
	screenshotprogress "mediainfo/internal/screenshot/progress"
	screenshotsubtitle "mediainfo/internal/screenshot/subtitle"
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

func (r *screenshotRunner) alignToSubtitle(requested float64) float64 {
	if !r.subtitleState.IndexBuilt || len(r.subtitleState.Index) == 0 {
		return requested
	}
	epsilon := 2.0
	if r.subtitle.Codec == "hdmv_pgs_subtitle" || r.subtitle.Codec == "dvd_subtitle" {
		epsilon = r.settings.SearchBack
	}
	if snapped, ok := screenshotsubtitle.SnapFromIndex(requested, r.subtitleState.Index, epsilon); ok {
		r.logf("[信息] 字幕对齐: 请求 %.3fs → 对齐到字幕 %.3fs", requested, snapped)
		return snapped
	}
	return requested
}

func (r *screenshotRunner) clampToDuration(value float64) float64 {
	if r.media.Duration > 0 && value >= r.media.Duration {
		return r.media.Duration - 1
	}
	if value < 0 { return 0 }
	return value
}

func (r *screenshotRunner) resolveUniqueScreenshotSecond(requested, aligned float64, usedSeconds map[int]struct{}) (float64, bool, bool) {
	second := int(aligned)
	if _, used := usedSeconds[second]; !used {
		return aligned, false, true
	}
	for offset := 1; offset < 10; offset++ {
		for _, candidate := range []float64{float64(second + offset), float64(second - offset)} {
			if candidate < 0 { continue }
			if _, used := usedSeconds[int(candidate)]; !used {
				return candidate + (aligned - float64(second)), true, true
			}
		}
	}
	return 0, false, false
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