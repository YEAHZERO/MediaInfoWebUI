package screenshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	screenshotprogress "mediainfo/internal/screenshot/progress"
	screenshottimestamps "mediainfo/internal/screenshot/timestamps"
	"mediainfo/internal/system"
	"mediainfo/internal/taskprogress"
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

func (r *screenshotRunner) clampToDuration(value float64) float64 {
	if value < 0 {
		return 0
	}
	if r.media.Duration > 0 && value > r.media.Duration {
		return r.media.Duration
	}
	return value
}

func (r *screenshotRunner) captureScreenshot(aligned float64, outputPath string) error {
	sourcePath := r.sourcePath
	if r.subtitleState.DVDResult != nil && r.subtitleState.DVDResult.SelectedVOBPath != "" {
		sourcePath = r.subtitleState.DVDResult.SelectedVOBPath
	}

	coarseBack := r.renderCoarseBack()
	_, fineSecond, coarseHMS := r.splitCaptureTimeline(aligned, coarseBack)

	if r.subtitle.Mode != "none" && r.isSupportedBitmapSubtitle() {
		return r.captureBitmapScreenshot(sourcePath, coarseHMS, fineSecond, outputPath)
	}
	return r.captureFrameDirect(sourcePath, coarseHMS, fineSecond, outputPath)
}

func (r *screenshotRunner) captureFrameDirect(sourcePath, coarseHMS string, fineSecond float64, outputPath string) error {
	args := []string{
		"-y",
		"-v", "error",
		"-fflags", "+genpts",
		"-ss", coarseHMS,
		"-probesize", r.settings.ProbeSize,
		"-analyzeduration", r.settings.Analyze,
		"-i", sourcePath,
		"-ss", screenshottimestamps.FormatSeconds(fineSecond),
		"-map", "0:v:0",
		"-frames:v", "1",
		"-an",
	}

	filterChain := joinFilters(
		r.buildTextSubtitleFilter(),
		strings.TrimSpace(r.render.ColorChain),
		r.displayAspectFilter(),
	)
	if filterChain != "" {
		args = append(args, "-vf", filterChain)
	}

	if r.variant == "jpg" {
		args = append(args, "-vcodec", "mjpeg", "-qscale:v", "3")
	} else {
		args = append(args, "-vcodec", "png", "-compression_level", "1")
	}

	args = append(args, "-y", outputPath)

	_, stderr, err := system.RunCommand(r.ctx, r.tools.FFmpegBin, args...)
	if err != nil {
		return parseFFmpegError(err, stderr, filterChain, sourcePath)
	}
	return nil
}

func parseFFmpegError(err error, stderr, filterChain, sourcePath string) error {
	msg := system.BestErrorMessage(err, stderr, "")
	return &ffmpegCaptureError{message: msg, filterChain: filterChain, source: sourcePath}
}

type ffmpegCaptureError struct {
	message     string
	filterChain string
	source      string
}

func (e *ffmpegCaptureError) Error() string {
	return "ffmpeg 截图失败"
}

func (e *ffmpegCaptureError) Unwrap() string {
	return e.message
}

func (r *screenshotRunner) captureBitmapScreenshot(sourcePath, coarseHMS string, fineSecond float64, outputPath string) error {
	overlayDir, err := os.MkdirTemp("", "screenshot-bitmap-overlay-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(overlayDir)

	baseFrame := filepath.Join(overlayDir, "base.png")
	if err := r.captureFrameDirect(sourcePath, coarseHMS, fineSecond, baseFrame); err != nil {
		return err
	}

	filterComplex := r.buildPGSRenderFilterComplex()
	filterComplex = strings.ReplaceAll(filterComplex, "[0:v:0]", "["+baseFrame+":v:0]")

	args := []string{
		"-y",
		"-v", "error",
		"-i", baseFrame,
		"-i", sourcePath,
		"-filter_complex", filterComplex,
		"-map", "[out]",
		"-frames:v", "1",
	}

	if r.variant == "jpg" {
		args = append(args, "-vcodec", "mjpeg", "-qscale:v", "3")
	} else {
		args = append(args, "-vcodec", "png", "-compression_level", "1")
	}

	args = append(args, "-y", outputPath)

	_, stderr, err := system.RunCommand(r.ctx, r.tools.FFmpegBin, args...)
	if err != nil {
		return fmt.Errorf("ffmpeg 位图截图覆盖失败: %s", system.BestErrorMessage(err, stderr, ""))
	}
	return nil
}

func generateScreenshotTimestampsFromSource(ctx context.Context, sourcePath string, count int) []float64 {
	ffprobe, err := system.ResolveBin("FFPROBE_BIN", "ffprobe")
	if err != nil {
		return fallbackTimestamps(count)
	}

	duration, err := screenshottimestamps.ProbeDuration(ctx, ffprobe, sourcePath)
	if err != nil || duration <= 0 {
		return fallbackTimestamps(count)
	}

	return generateDistributedTimestamps(duration, count)
}

func fallbackTimestamps(count int) []float64 {
	result := make([]float64, count)
	for i := 0; i < count; i++ {
		result[i] = float64(i*10 + 5)
	}
	return result
}

func generateDistributedTimestamps(duration float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	if duration <= 0 {
		return []float64{0}
	}
	if count == 1 {
		return []float64{duration / 2}
	}

	seconds := make([]float64, count)
	step := duration / float64(count+1)
	for i := 0; i < count; i++ {
		seconds[i] = step * float64(i+1)
	}
	return seconds
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