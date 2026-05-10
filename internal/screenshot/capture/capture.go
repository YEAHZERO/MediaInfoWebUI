package capture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mediainfo/internal/screenshot/render"
	"mediainfo/internal/screenshot/subtitle"
	"mediainfo/internal/screenshot/timestamps"
	"mediainfo/internal/system"
)

type CaptureOptions struct {
	SourcePath     string
	OutputDir      string
	Variant        string
	SubtitleMode   string
	Count          int
	OnProgress     func(phase string, current, total int, message string)
	CoarseBack     float64
}

type CaptureResult struct {
	Files      []FileInfo
	Logs       string
	ColorSpace *render.ColorSpaceInfo
}

type FileInfo struct {
	Path string
	Name string
	Size int64
}

func Capture(ctx context.Context, opts CaptureOptions) (*CaptureResult, error) {
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
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

	duration, err := timestamps.ProbeDuration(ctx, ffprobe, opts.SourcePath)
	if err != nil {
		return nil, err
	}

	csInfo, _ := render.ProbeColorSpace(ctx, ffprobe, opts.SourcePath)
	
	seconds := timestamps.GenerateRandom(duration, opts.Count)
	
	handler, subtitleIndex := subtitle.GetSubtitleHandler(ctx, ffprobe, opts.SourcePath, opts.SubtitleMode)
	format := subtitle.GetOutputFormat(opts.Variant)

	var logs strings.Builder
	files := make([]FileInfo, 0, len(seconds))

	for i, second := range seconds {
		filename := fmt.Sprintf("screenshot_%02d%s", i+1, format.Extension())
		outputPath := filepath.Join(opts.OutputDir, filename)

		if opts.OnProgress != nil {
			opts.OnProgress("capture", i+1, opts.Count, fmt.Sprintf("capturing %d/%d at %.1fs", i+1, opts.Count, second))
		}

		coarseSecond := second - opts.CoarseBack
		if coarseSecond < 0 {
			coarseSecond = 0
		}
		fineSecond := second - coarseSecond

		var captureErr error
		if handler.NeedsBitmapOverlay() {
			captureErr = captureBitmapOverlay(ctx, ffmpeg, ffprobe, opts.SourcePath, outputPath, coarseSecond, fineSecond, handler, format, csInfo)
		} else {
			captureErr = captureFrame(ctx, ffmpeg, opts.SourcePath, outputPath, coarseSecond, fineSecond, handler, format, subtitleIndex, csInfo)
		}

		logs.WriteString(captureLog(captureErr, i+1, len(seconds)))
		if captureErr == nil {
			if info, err := os.Stat(outputPath); err == nil {
				files = append(files, FileInfo{
					Path: outputPath,
					Name: filename,
					Size: info.Size(),
				})
			}
		}
	}

	if len(files) == 0 {
		return &CaptureResult{Logs: logs.String(), ColorSpace: csInfo},
			fmt.Errorf("all %d screenshot captures failed", len(seconds))
	}

	return &CaptureResult{
		Files:      files,
		Logs:       logs.String(),
		ColorSpace: csInfo,
	}, nil
}

func captureFrame(ctx context.Context, ffmpeg, source, output string, coarseSecond, fineSecond float64, handler subtitle.SubtitleHandler, format subtitle.OutputFormat, subtitleIndex int, csInfo *render.ColorSpaceInfo) error {
	args := []string{
		"-y",
		"-v", "error",
		"-fflags", "+genpts",
		"-ss", timestamps.FormatTimestamp(coarseSecond),
		"-i", source,
		"-ss", timestamps.FormatSeconds(fineSecond),
		"-map", "0:v:0",
		"-frames:v", "1",
		"-an",
	}

	filterChain := buildFilterChain(handler, subtitleIndex, fineSecond, csInfo)
	if filterChain != "" {
		args = append(args, "-vf", filterChain)
	}

	args = append(args, format.CodecArgs()...)
	args = append(args, handler.BuildOutputArgs(filepath.Dir(output), filepath.Base(output))...)

	_, stderr, err := system.RunCommand(ctx, ffmpeg, args...)
	if err != nil {
		return fmt.Errorf("ffmpeg: %s", system.BestErrorMessage(err, stderr, ""))
	}
	return nil
}

func captureBitmapOverlay(ctx context.Context, ffmpeg, ffprobe, source, output string, coarseSecond, fineSecond float64, handler subtitle.SubtitleHandler, format subtitle.OutputFormat, csInfo *render.ColorSpaceInfo) error {
	overlayDir, err := os.MkdirTemp("", "screenshot-overlay-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(overlayDir)

	baseFrame := filepath.Join(overlayDir, "base.png")
	if err := captureFrame(ctx, ffmpeg, source, baseFrame, coarseSecond, fineSecond, &subtitle.NoSubtitleHandler{}, &subtitle.PNGFormat{}, -1, csInfo); err != nil {
		return err
	}

	vw, vh := probeDimensions(ffprobe, baseFrame)
	filterComplex := fmt.Sprintf("[1:v]format=yuva420p[sub];[0:v][sub]overlay=0:%d", vh-10)
	if vw > 0 && vh > 0 {
		filterComplex = fmt.Sprintf("[1:v]scale=%d:-2[sub];[0:v][sub]overlay=0:%d", vw, vh-10)
	}

	args := []string{
		"-y", "-v", "error",
		"-ss", timestamps.FormatTimestamp(coarseSecond), "-i", source,
		"-ss", timestamps.FormatSeconds(fineSecond), "-i", baseFrame,
		"-filter_complex", filterComplex,
		"-map", "0:s:0?",
	}
	args = append(args, format.CodecArgs()...)
	args = append(args, output)

	_, stderr, err := system.RunCommand(ctx, ffmpeg, args...)
	if err != nil {
		return fmt.Errorf("bitmap overlay: %s", system.BestErrorMessage(err, stderr, ""))
	}
	return nil
}

func buildFilterChain(handler subtitle.SubtitleHandler, subtitleIndex int, time float64, csInfo *render.ColorSpaceInfo) string {
	var filters []string

	if csInfo != nil && render.NeedsToneMapping(csInfo) {
		filters = append(filters, buildToneMappingFilter(csInfo))
	}

	if chain := handler.BuildFilterChain(time, subtitleIndex); chain != "" {
		filters = append(filters, chain)
	}

	if len(filters) == 0 {
		return ""
	}
	return strings.Join(filters, ",")
}

func buildToneMappingFilter(csInfo *render.ColorSpaceInfo) string {
	if csInfo.DolbyVision {
		return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
	}
	if strings.Contains(strings.ToLower(csInfo.Transfer), "smpte2084") {
		return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
	}
	if strings.Contains(strings.ToLower(csInfo.Transfer), "arib") {
		return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
	}
	return "libplacebo=format=rgb48:tonemap=bt2390:peak=100"
}

func probeDimensions(ffprobe, filePath string) (int, int) {
	stdout, _, _ := system.RunCommand(context.Background(), ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width:stream=height",
		"-of", "csv=p=0",
		filePath,
	)
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		fields := strings.Split(strings.TrimSpace(line), ",")
		if len(fields) >= 2 {
			var w, h int
			fmt.Sscanf(fields[0], "%d", &w)
			fmt.Sscanf(fields[1], "%d", &h)
			return w, h
		}
	}
	return 0, 0
}

func captureLog(err error, current, total int) string {
	if err != nil {
		return fmt.Sprintf("  [error] screenshot %d/%d failed: %s\n", current, total, err.Error())
	}
	return fmt.Sprintf("  [ok] screenshot %d/%d\n", current, total)
}