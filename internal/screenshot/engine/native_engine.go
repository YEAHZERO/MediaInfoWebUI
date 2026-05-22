//go:build native

package engine

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mediainfo/internal/system"
)

var subtitleHandlers = map[string]SubtitleHandler{
	"text": &textSubtitleHandler{},
	"pgs":  &pgsSubtitleHandler{},
	"dvd":  &dvdSubtitleHandler{},
	"":     &noSubtitleHandler{},
}

var outputFormats = map[string]OutputFormat{
	"png": &pngFormat{},
	"jpg": &jpgFormat{},
}

type SubtitleHandler interface {
	BuildFilterChain(time float64, subtitleIndex int) string
	BuildOutputArgs(outputDir, filename string) []string
	SubtitleArgs() []string
}

type OutputFormat interface {
	Extension() string
	CodecArgs() []string
}

type textSubtitleHandler struct {
	sourcePath string
}

func (h *textSubtitleHandler) BuildFilterChain(time float64, subtitleIndex int) string {
	if subtitleIndex < 0 {
		return ""
	}
	return fmt.Sprintf("subtitles='%s':si=%d", escapeFilterPath(h.sourcePath), subtitleIndex)
}

func (h *textSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{filepath.Join(outputDir, filename)}
}

func (h *textSubtitleHandler) SubtitleArgs() []string {
	return nil
}

type pgsSubtitleHandler struct{}

func (h *pgsSubtitleHandler) BuildFilterChain(_ float64, _ int) string {
	return "copy"
}

func (h *pgsSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "0",
		filepath.Join(outputDir, "overlay.mkv"),
	}
}

func (h *pgsSubtitleHandler) SubtitleArgs() []string {
	return nil
}

type dvdSubtitleHandler struct{}

func (h *dvdSubtitleHandler) BuildFilterChain(_ float64, _ int) string {
	return "copy"
}

func (h *dvdSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "0",
		filepath.Join(outputDir, "overlay.mkv"),
	}
}

func (h *dvdSubtitleHandler) SubtitleArgs() []string {
	return nil
}

type noSubtitleHandler struct{}

func (h *noSubtitleHandler) BuildFilterChain(_ float64, _ int) string {
	return ""
}

func (h *noSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{filepath.Join(outputDir, filename)}
}

func (h *noSubtitleHandler) SubtitleArgs() []string {
	return nil
}

type pngFormat struct{}

func (f *pngFormat) Extension() string {
	return ".png"
}

func (f *pngFormat) CodecArgs() []string {
	return []string{
		"-vcodec", "png",
		"-compression_level", "1",
	}
}

type jpgFormat struct{}

func (f *jpgFormat) Extension() string {
	return ".jpg"
}

func (f *jpgFormat) CodecArgs() []string {
	return []string{
		"-vcodec", "mjpeg",
		"-qscale:v", "3",
	}
}

type nativeEngine struct{}

func newNativeEngine() *nativeEngine {
	return &nativeEngine{}
}

func (e *nativeEngine) newSubtitleHandler(subtitleType, sourcePath string) SubtitleHandler {
	if subtitleType == "" {
		return &noSubtitleHandler{}
	}
	return &textSubtitleHandler{sourcePath: sourcePath}
}

func (e *nativeEngine) Capture(ctx context.Context, opts CaptureOptions) (*CaptureResult, error) {
	hb := startHeartbeat(opts.OnProgress)

	sourcePath := opts.SourcePath
	outputDir := opts.OutputDir

	hb.update(PhaseProbe, 0, opts.Count, "probing source")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	ffprobe, err := system.ResolveBin("FFPROBE_BIN", "ffprobe")
	if err != nil {
		return nil, err
	}

	ffmpeg, err := system.ResolveBin("FFMPEG_BIN", "ffmpeg")
	if err != nil {
		return nil, err
	}

	hb.update(PhaseTimestamps, 0, opts.Count, "probing duration")
	duration, err := probeDuration(ctx, ffprobe, sourcePath)
	if err != nil {
		return nil, err
	}

	hb.update(PhaseTimestamps, 0, opts.Count, "detecting color space")
	csInfo, _ := DetectColorSpace(ctx, sourcePath)

	hb.update(PhaseTimestamps, 0, opts.Count, "detecting subtitles")
	seconds := buildRandomSecondsFloat(duration, opts.Count)

	subtitleType, subtitleIndex := e.detectSubtitleWithIndex(ctx, sourcePath, ffprobe)

	handler := e.newSubtitleHandler(subtitleType, sourcePath)
	if opts.SubtitleMode == SubtitleModeOff {
		handler = &noSubtitleHandler{}
		subtitleIndex = -1
	}

	format := outputFormats[opts.Variant]

	var logs strings.Builder
	files := make([]ScreenshotFileInfo, 0, len(seconds))

	for i, second := range seconds {
		ts := int(math.Round(second))
		filename := fmt.Sprintf("%dmin%02ds%s", ts/60, ts%60, format.Extension())
		if ts < 60 {
			filename = fmt.Sprintf("%ds%s", ts, format.Extension())
		}
		outputPath := filepath.Join(outputDir, filename)

		hb.update(PhaseCapture, i+1, opts.Count, fmt.Sprintf("capturing %d/%d at %.1fs", i+1, opts.Count, second))

		coarseBack := CoarseBackDefault
		if subtitleType == "pgs" || subtitleType == "dvd" {
			coarseBack = coarseBackPGS
		}
		coarseSecond, fineSecond := splitTimeline(second, coarseBack)
		coarseHMS := formatTimestamp(coarseSecond)

		var captureErr error
		captureErr = e.captureFrame(ctx, ffmpeg, sourcePath, outputPath, coarseHMS, fineSecond, handler, format, subtitleIndex, csInfo)

		logs.WriteString(e.captureLog(captureErr, i+1, len(seconds)))
		if captureErr == nil {
			info, _ := os.Stat(outputPath)
			if info != nil {
				files = append(files, ScreenshotFileInfo{
					Path: outputPath,
					Name: filename,
					Size: info.Size(),
				})
			}

			hb.update(PhaseCompress, i+1, opts.Count, "checking file size")
			if result, cerr := CompressIfNeeded(ctx, outputPath, 0, StrategyAuto); cerr == nil && result.Compressed {
				logs.WriteString(fmt.Sprintf("  [compress] %s: %s\n", filepath.Base(outputPath), result.Method))
			}
		}
	}

	if len(files) == 0 {
		hb.doneMsg("all captures failed")
		return &CaptureResult{Logs: logs.String(), ColorSpace: csInfo},
			fmt.Errorf("all %d screenshot captures failed", len(seconds))
	}

	hb.doneMsg(fmt.Sprintf("completed %d screenshots in %s", len(files), hb.elapsed()))
	hb.stop()

	return &CaptureResult{
		Files:      files,
		Logs:       logs.String(),
		ColorSpace: csInfo,
	}, nil
}

func (e *nativeEngine) DetectColorSpace(ctx context.Context, sourcePath string) (*ColorSpaceInfo, error) {
	return DetectColorSpace(ctx, sourcePath)
}

func (e *nativeEngine) CompressIfNeeded(ctx context.Context, path string, threshold int64, strategy string) (*CompressionResult, error) {
	return CompressIfNeeded(ctx, path, threshold, strategy)
}

func (e *nativeEngine) captureFrame(ctx context.Context, ffmpeg, source, output, coarseHMS string, fineSecond float64, handler SubtitleHandler, format OutputFormat, subtitleIndex int, csInfo *ColorSpaceInfo) error {
	args := buildCaptureFrameArgs(coarseHMS, source, fineSecond)

	filterChain := e.buildVideoFilterChain(handler, subtitleIndex, fineSecond, csInfo)
	if usesLibplaceboColorspace(csInfo) {
		return e.captureWithLibplaceboFallback(ctx, ffmpeg, args, filterChain, handler, format, output, csInfo)
	}

	if filterChain != "" {
		args = append(args, "-vf", filterChain)
	}

	args = append(args, format.CodecArgs()...)
	args = append(args, "-y")
	args = append(args, handler.BuildOutputArgs(filepath.Dir(output), filepath.Base(output))...)

	_, stderr, err := system.RunCommand(ctx, ffmpeg, args...)
	if err != nil {
		return fmt.Errorf("ffmpeg 失败 | 滤镜: %s | 路径: %s | 错误: %s", filterChain, source, system.BestErrorMessage(err, stderr, ""))
	}
	return nil
}

func buildCaptureFrameArgs(coarseHMS string, source string, fineSecond float64) []string {
	return []string{
		"-y",
		"-v", "error",
		"-fflags", "+genpts",
		"-ss", coarseHMS,
		"-i", source,
		"-ss", fmt.Sprintf("%.3f", fineSecond),
		"-map", "0:v:0",
		"-frames:v", "1",
		"-an",
	}
}

func (e *nativeEngine) captureWithLibplaceboFallback(ctx context.Context, ffmpeg string, args []string, filterChain string, handler SubtitleHandler, format OutputFormat, output string, csInfo *ColorSpaceInfo) error {
	libplaceboChain := filterChain
	if libplaceboChain == "" {
		libplaceboChain = e.buildToneMappingFilter(csInfo)
	}

	captureArgs := append([]string{}, args...)
	captureArgs = append(captureArgs, "-vf", libplaceboChain)
	captureArgs = append(captureArgs, format.CodecArgs()...)
	captureArgs = append(captureArgs, "-y")
	captureArgs = append(captureArgs, handler.BuildOutputArgs(filepath.Dir(output), filepath.Base(output))...)

	_, stderr, err := system.RunCommand(ctx, ffmpeg, captureArgs...)
	if err == nil {
		return nil
	}

	errMsg := system.BestErrorMessage(err, stderr, "")
	if !isLibplaceboRenderCrashMessage(errMsg) {
		return fmt.Errorf("ffmpeg 失败 | 滤镜: %s | 错误: %s", libplaceboChain, errMsg)
	}

	fallbackFilter := buildFallbackToneMappingFilter(csInfo)
	fallbackArgs := append([]string{}, args...)
	if filterChain != "" {
		existingParts := strings.SplitN(filterChain, ",", 2)
		if len(existingParts) == 2 {
			fallbackArgs = append(fallbackArgs, "-vf", fallbackFilter+","+existingParts[1])
		} else {
			fallbackArgs = append(fallbackArgs, "-vf", fallbackFilter)
		}
	} else {
		fallbackArgs = append(fallbackArgs, "-vf", fallbackFilter)
	}
	fallbackArgs = append(fallbackArgs, format.CodecArgs()...)
	fallbackArgs = append(fallbackArgs, "-y")
	fallbackArgs = append(fallbackArgs, handler.BuildOutputArgs(filepath.Dir(output), filepath.Base(output))...)

	_, stderr2, err2 := system.RunCommand(ctx, ffmpeg, fallbackArgs...)
	if err2 != nil {
		return fmt.Errorf("ffmpeg fallback 失败 | 滤镜: %s | 错误: %s", fallbackFilter, system.BestErrorMessage(err2, stderr2, ""))
	}
	return nil
}

func usesLibplaceboColorspace(csInfo *ColorSpaceInfo) bool {
	if csInfo == nil {
		return false
	}
	if csInfo.DolbyVision {
		return true
	}
	if strings.Contains(csInfo.Transfer, "smpte2084") {
		return true
	}
	if strings.Contains(csInfo.Transfer, "arib-std-b67") || strings.Contains(csInfo.Transfer, "arib") {
		return true
	}
	return false
}

func isLibplaceboRenderCrashMessage(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "libplacebo") ||
		strings.Contains(lower, "vkcreateinstance") ||
		strings.Contains(lower, "vulkan") ||
		strings.Contains(lower, "failed to create vulkan device") ||
		strings.Contains(lower, "failed to initialize vulkan")
}

func buildFallbackToneMappingFilter(csInfo *ColorSpaceInfo) string {
	if csInfo.DolbyVision {
		return "zscale=transfer=linear,tonemap=hable,zscale=transfer=bt709"
	}
	return "zscale=transfer=linear,tonemap=hable,zscale=transfer=bt709"
}

func (e *nativeEngine) buildVideoFilterChain(handler SubtitleHandler, subtitleIndex int, time float64, csInfo *ColorSpaceInfo) string {
	var filters []string

	if csInfo != nil && NeedsToneMapping(csInfo) {
		toneMapping := e.buildToneMappingFilter(csInfo)
		if toneMapping != "" {
			filters = append(filters, toneMapping)
		}
	}

	if chain := handler.BuildFilterChain(time, subtitleIndex); chain != "" && chain != "copy" {
		filters = append(filters, chain)
	}

	if len(filters) == 0 {
		return ""
	}

	result := filters[0]
	for i := 1; i < len(filters); i++ {
		result = result + "," + filters[i]
	}
	return result
}

func (e *nativeEngine) buildToneMappingFilter(csInfo *ColorSpaceInfo) string {
	if csInfo == nil {
		return ""
	}

	if csInfo.DolbyVision {
		return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
	}

	if strings.Contains(csInfo.Transfer, "smpte2084") {
		return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
	}

	if strings.Contains(csInfo.Transfer, "arib-std-b67") || strings.Contains(csInfo.Transfer, "arib") {
		return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
	}

	return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
}

func (e *nativeEngine) captureBitmapSubtitle(ctx context.Context, ffmpeg, ffprobe, source, output string, second float64, coarseHMS string, fineSecond float64, handler SubtitleHandler, format OutputFormat, csInfo *ColorSpaceInfo) error {
	overlayDir, err := os.MkdirTemp("", "bitmap-overlay-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(overlayDir)

	baseFrame := filepath.Join(overlayDir, "base.png")
	if err := e.captureFrame(ctx, ffmpeg, source, baseFrame, coarseHMS, fineSecond, &noSubtitleHandler{}, format, -1, csInfo); err != nil {
		return err
	}

	args := e.buildBitmapOverlayArgs(ffmpeg, ffprobe, source, baseFrame, coarseHMS, fineSecond, output, format)
	_, stderr, err := system.RunCommand(ctx, ffmpeg, args...)
	if err != nil {
		return fmt.Errorf("bitmap subtitle overlay: %s", system.BestErrorMessage(err, stderr, ""))
	}
	return nil
}

func (e *nativeEngine) buildBitmapOverlayArgs(ffmpeg, ffprobe, source, baseFrame, coarseHMS string, fineSecond float64, output string, format OutputFormat) []string {
	vw, vh := probeVideoDimensions(ffprobe, baseFrame)
	filterComplex := fmt.Sprintf("[1:v]format=yuva420p[sub];[0:v][sub]overlay=0:%d", vh-10)
	if vw > 0 && vh > 0 {
		filterComplex = fmt.Sprintf("[1:v]scale=%d:-2[sub];[0:v][sub]overlay=0:%d", vw, vh-10)
	}

	return []string{
		"-y", "-v", "error",
		"-ss", coarseHMS, "-i", source,
		"-ss", fmt.Sprintf("%.3f", fineSecond), "-i", baseFrame,
		"-filter_complex", filterComplex,
		"-map", "0:s:0?",
	}
}

func (e *nativeEngine) captureLog(err error, current, total int) string {
	if err != nil {
		return fmt.Sprintf("  [error] screenshot %d/%d failed: %s\n", current, total, err.Error())
	}
	return fmt.Sprintf("  [ok] screenshot %d/%d\n", current, total)
}

func (e *nativeEngine) detectSubtitleWithIndex(ctx context.Context, sourcePath, ffprobe string) (string, int) {
	subtitleType := detectSubtitleType(ctx, sourcePath, ffprobe)
	if subtitleType == "" {
		return "", -1
	}

	subtitleIndex := detectSubtitleRelativeIndex(ctx, sourcePath, ffprobe)
	return subtitleType, subtitleIndex
}

func detectSubtitleType(ctx context.Context, sourcePath, ffprobe string) string {
	if ffprobe == "" {
		var err error
		ffprobe, err = system.ResolveBin("FFPROBE_BIN", "ffprobe")
		if err != nil {
			return ""
		}
	}

	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=codec_name",
		"-of", "csv=p=0",
		sourcePath,
	)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return ""
	}

	for _, line := range strings.Split(stdout, "\n") {
		codec := strings.TrimSpace(line)
		switch {
		case strings.Contains(codec, "pgssub"), strings.Contains(codec, "hdmv_pgs_subtitle"):
			return "pgs"
		case strings.Contains(codec, "dvdsub"), strings.Contains(codec, "dvd_subtitle"):
			return "dvd"
		case strings.Contains(codec, "subrip"), strings.Contains(codec, "srt"),
			strings.Contains(codec, "ass"), strings.Contains(codec, "ssa"),
			strings.Contains(codec, "mov_text"):
			return "text"
		}
	}
	return ""
}

func detectSubtitleRelativeIndex(ctx context.Context, sourcePath, ffprobe string) int {
	if ffprobe == "" {
		var err error
		ffprobe, err = system.ResolveBin("FFPROBE_BIN", "ffprobe")
		if err != nil {
			return -1
		}
	}

	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name",
		"-of", "csv=p=0",
		sourcePath,
	)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return -1
	}

	subtitleRelIndex := 0
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			codec := strings.TrimSpace(parts[1])
			switch {
			case strings.Contains(codec, "pgssub"), strings.Contains(codec, "hdmv_pgs_subtitle"),
				strings.Contains(codec, "dvdsub"), strings.Contains(codec, "dvd_subtitle"),
				strings.Contains(codec, "subrip"), strings.Contains(codec, "srt"),
				strings.Contains(codec, "ass"), strings.Contains(codec, "ssa"),
				strings.Contains(codec, "mov_text"):
				return subtitleRelIndex
			}
		}
		subtitleRelIndex++
	}
	return -1
}

func probeVideoDimensions(ffprobe, path string) (int, int) {
	stdout, _, err := system.RunCommand(context.Background(), ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "default=noprint_wrappers=1",
		path,
	)
	if err != nil {
		return 0, 0
	}
	vw, vh := 0, 0
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "width=") {
			fmt.Sscanf(line, "width=%d", &vw)
		}
		if strings.HasPrefix(line, "height=") {
			fmt.Sscanf(line, "height=%d", &vh)
		}
	}
	return vw, vh
}

type subtitleStream struct {
	Index      int
	Codec      string
	Language   string
	Forced     bool
	IsDefault  bool
}

func detectAllSubtitleStreams(ctx context.Context, sourcePath, ffprobe string) []subtitleStream {
	if ffprobe == "" {
		var err error
		ffprobe, err = system.ResolveBin("FFPROBE_BIN", "ffprobe")
		if err != nil {
			return nil
		}
	}

	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name,language:stream=disposition",
		"-of", "csv=p=0",
		sourcePath,
	)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return nil
	}

	var streams []subtitleStream
	relIndex := 0
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			s := subtitleStream{
				Index:    relIndex,
				Codec:    strings.TrimSpace(parts[1]),
				Language: "",
			}
			if len(parts) >= 3 {
				s.Language = strings.TrimSpace(parts[2])
			}
			if len(parts) >= 4 {
				disposition := strings.TrimSpace(parts[3])
				s.Forced = strings.Contains(disposition, "forced")
				s.IsDefault = strings.Contains(disposition, "default")
			}
			streams = append(streams, s)
		}
		relIndex++
	}

	sort.Slice(streams, func(i, j int) bool {
		if streams[i].Forced != streams[j].Forced {
			return streams[i].Forced
		}
		if streams[i].IsDefault != streams[j].IsDefault {
			return streams[i].IsDefault
		}
		if streams[i].Language == "chi" || streams[i].Language == "chs" || streams[i].Language == "zh" {
			return true
		}
		return false
	})

	return streams
}

func escapeFilterPath(path string) string {
	path = strings.ReplaceAll(path, `\`, `\\`)
	path = strings.ReplaceAll(path, `'`, `\'`)
	path = strings.ReplaceAll(path, `:`, `\:`)
	path = strings.ReplaceAll(path, `,`, `\,`)
	path = strings.ReplaceAll(path, `;`, `\;`)
	path = strings.ReplaceAll(path, `[`, `\[`)
	path = strings.ReplaceAll(path, `]`, `\]`)
	return path
}
