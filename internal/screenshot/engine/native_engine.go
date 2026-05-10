//go:build native

package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	BuildFilterChain(time float64) string
	BuildOutputArgs(outputDir, filename string) []string
}

type OutputFormat interface {
	Extension() string
	CodecArgs() []string
}

type textSubtitleHandler struct{}

func (h *textSubtitleHandler) BuildFilterChain(time float64) string {
	return fmt.Sprintf("subtitles=%.6f", time)
}

func (h *textSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{filepath.Join(outputDir, filename)}
}

type pgsSubtitleHandler struct{}

func (h *pgsSubtitleHandler) BuildFilterChain(_ float64) string {
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

type dvdSubtitleHandler struct{}

func (h *dvdSubtitleHandler) BuildFilterChain(_ float64) string {
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

type noSubtitleHandler struct{}

func (h *noSubtitleHandler) BuildFilterChain(_ float64) string {
	return ""
}

func (h *noSubtitleHandler) BuildOutputArgs(outputDir, filename string) []string {
	return []string{filepath.Join(outputDir, filename)}
}

type pngFormat struct{}

func (f *pngFormat) Extension() string {
	return ".png"
}

func (f *pngFormat) CodecArgs() []string {
	return []string{
		"-vcodec", "png",
		"-compression_level", "3",
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

	hb.update(PhaseTimestamps, 0, opts.Count, "probing duration")
	duration, err := probeDuration(ctx, ffprobe, sourcePath)
	if err != nil {
		return nil, err
	}

	hb.update(PhaseTimestamps, 0, opts.Count, "detecting color space")
	csInfo, _ := DetectColorSpace(ctx, sourcePath)

	hb.update(PhaseTimestamps, 0, opts.Count, "detecting subtitles")
	seconds := buildRandomSecondsFloat(duration, opts.Count)
	subtitleType := e.detectSubtitleTypeVerbose(ctx, sourcePath, hb)
	handler := subtitleHandlers[subtitleType]
	format := outputFormats[opts.Variant]

	var logs strings.Builder
	files := make([]string, 0, len(seconds))

	ffmpeg, err := system.ResolveBin("FFMPEG_BIN", "ffmpeg")
	if err != nil {
		return nil, err
	}

	for i, second := range seconds {
		filename := fmt.Sprintf("screenshot_%02d%s", i+1, format.Extension())
		outputPath := filepath.Join(outputDir, filename)

		hb.update(PhaseCapture, i+1, opts.Count, fmt.Sprintf("capturing %d/%d at %.1fs", i+1, opts.Count, second))

		coarseSecond, fineSecond := splitTimeline(second, CoarseBackDefault)
		coarseHMS := formatTimestamp(coarseSecond)

		if subtitleType == "pgs" || subtitleType == "dvd" {
			err = e.captureBitmapSubtitle(ctx, ffmpeg, ffprobe, sourcePath, outputPath, second, coarseHMS, fineSecond, handler, format)
		} else {
			err = e.captureFrame(ctx, ffmpeg, sourcePath, outputPath, coarseHMS, fineSecond, handler, format)
		}

		logs.WriteString(e.captureLog(err, i+1, len(seconds)))
		if err == nil {
			files = append(files, outputPath)

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

func (e *nativeEngine) captureFrame(ctx context.Context, ffmpeg, source, output, coarseHMS string, fineSecond float64, handler SubtitleHandler, format OutputFormat) error {
	args := []string{
		ffmpeg, "-y",
		"-ss", coarseHMS,
		"-i", source,
		"-ss", fmt.Sprintf("%.3f", fineSecond),
		"-map", "0:v:0",
		"-frames:v", "1",
		"-an",
	}

	if chain := handler.BuildFilterChain(float64(0)); chain != "" && chain != "copy" {
		args = append(args, "-vf", chain)
	}

	args = append(args, format.CodecArgs()...)
	args = append(args, "-y")
	args = append(args, handler.BuildOutputArgs(filepath.Dir(output), filepath.Base(output))...)

	_, _, err := system.RunCommand(ctx, args[0], args[1:]...)
	return err
}

func (e *nativeEngine) captureBitmapSubtitle(ctx context.Context, ffmpeg, ffprobe, source, output string, second float64, coarseHMS string, fineSecond float64, handler SubtitleHandler, format OutputFormat) error {
	overlayDir, err := os.MkdirTemp("", "pgs-overlay-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(overlayDir)

	overlayMKV := filepath.Join(overlayDir, "overlay.mkv")
	subtitleArgs := handler.BuildOutputArgs(overlayDir, "overlay.mkv")

	extractArgs := []string{
		ffmpeg, "-y",
		"-ss", coarseHMS,
		"-i", source,
		"-ss", fmt.Sprintf("%.3f", fineSecond),
		"-map", "0:s:0",
		"-frames:s", "1",
		"-c:s", "copy",
	}
	extractArgs = append(extractArgs, subtitleArgs...)

	if _, _, err := system.RunCommand(ctx, extractArgs[0], extractArgs[1:]...); err != nil {
		return fmt.Errorf("subtitle extraction: %w", err)
	}

	if _, err := os.Stat(overlayMKV); err != nil {
		return e.captureFrame(ctx, ffmpeg, source, output, coarseHMS, fineSecond, &noSubtitleHandler{}, format)
	}

	pngOverlay := filepath.Join(overlayDir, "overlay.png")
	convertArgs := []string{
		ffmpeg, "-y",
		"-i", overlayMKV,
		"-vframes", "1",
		"-compression_level", "3",
		pngOverlay,
	}
	if _, _, err := system.RunCommand(ctx, convertArgs[0], convertArgs[1:]...); err != nil {
		return e.captureFrame(ctx, ffmpeg, source, output, coarseHMS, fineSecond, &noSubtitleHandler{}, format)
	}

	baseFrame := filepath.Join(overlayDir, "base.png")
	if err := e.captureFrame(ctx, ffmpeg, source, baseFrame, coarseHMS, fineSecond, &noSubtitleHandler{}, format); err != nil {
		return err
	}

	compositeArgs := e.buildCompositeArgs(ffmpeg, ffprobe, baseFrame, pngOverlay, output, format)
	_, _, err = system.RunCommand(ctx, compositeArgs[0], compositeArgs[1:]...)
	return err
}

func (e *nativeEngine) buildCompositeArgs(ffmpeg, ffprobe, baseFrame, pngOverlay, output string, format OutputFormat) []string {
	vw, vh := probeVideoDimensions(ffprobe, baseFrame)
	ow, oh := probeVideoDimensions(ffprobe, pngOverlay)

	var filterComplex string
	if vw > 0 && vh > 0 && ow > 0 && oh > 0 {
		if ow == vw && oh <= vh {
			yOff := vh - oh - 10
			if yOff < 0 {
				yOff = 0
			}
			filterComplex = fmt.Sprintf("[0:v][1:v]overlay=0:%d", yOff)
		} else {
			filterComplex = fmt.Sprintf("[1:v]scale=%d:-2[sub];[0:v][sub]overlay=(W-w)/2:(H-h-10)", vw)
		}
	} else {
		filterComplex = "[0:v][1:v]overlay=(W-w)/2:(H-h-10)"
	}

	args := []string{
		ffmpeg, "-y",
		"-i", baseFrame, "-i", pngOverlay,
		"-filter_complex", filterComplex,
	}
	args = append(args, format.CodecArgs()...)
	args = append(args, output)
	return args
}

func (e *nativeEngine) captureLog(err error, current, total int) string {
	if err != nil {
		return fmt.Sprintf("  [error] screenshot %d/%d failed: %s\n", current, total, err.Error())
	}
	return fmt.Sprintf("  [ok] screenshot %d/%d\n", current, total)
}

func (e *nativeEngine) detectSubtitleTypeVerbose(ctx context.Context, sourcePath string, hb *heartbeat) string {
	hb.update(PhaseSubtitle, 0, 1, "probing subtitle streams")
	return detectSubtitleType(ctx, sourcePath)
}

func detectSubtitleType(ctx context.Context, sourcePath string) string {
	ffprobe, err := system.ResolveBin("FFPROBE_BIN", "ffprobe")
	if err != nil {
		return ""
	}

	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=codec_name:stream=index:stream=codec_type",
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