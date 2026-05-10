package engine

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"mediainfo/internal/media"
	"mediainfo/internal/system"
)

type scriptEngine struct {
	screenshotScriptDir string
}

func newScriptEngine() *scriptEngine {
	return &scriptEngine{
		screenshotScriptDir: "/usr/local/share/mediainfo/scripts",
	}
}

func (e *scriptEngine) Capture(ctx context.Context, opts CaptureOptions) (*CaptureResult, error) {
	hb := startHeartbeat(opts.OnProgress)
	hb.update(PhaseProbe, 0, opts.Count, "probing source")
	sourcePath, cleanup, err := media.ResolveScreenshotSource(ctx, opts.SourcePath)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	scriptPath, err := e.resolveScript(opts.Variant)
	if err != nil {
		return nil, err
	}

	hb.update(PhaseTimestamps, 0, opts.Count, "generating timestamps")
	csInfo, _ := DetectColorSpace(ctx, sourcePath)

	seconds, err := e.generateTimestamps(ctx, sourcePath, opts.Count)
	if err != nil {
		return nil, err
	}
	timestamps := make([]string, 0, len(seconds))
	for _, s := range seconds {
		timestamps = append(timestamps, fmt.Sprintf("%.3f", s))
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, err
	}

	subtitleArgs := e.subtitleModeArgs(opts.SubtitleMode)
	var logs strings.Builder

	for i, ts := range timestamps {
		hb.update(PhaseCapture, i+1, opts.Count, fmt.Sprintf("capturing screenshot %d/%d at %s", i+1, opts.Count, ts))

		args := append([]string{scriptPath}, subtitleArgs...)
		args = append(args, sourcePath, opts.OutputDir, ts)

		stdout, stderr, err := system.RunCommand(ctx, "bash", args...)
		logs.WriteString(system.CombineCommandOutput(stdout, stderr))
		if err != nil {
			hb.doneMsg(fmt.Sprintf("screenshot %d/%d failed", i+1, opts.Count))
			return &CaptureResult{Logs: logs.String(), ColorSpace: csInfo},
				fmt.Errorf("screenshot %d/%d failed: %s", i+1, opts.Count, system.BestErrorMessage(err, stderr, stdout))
		}
	}

	files, err := e.listScreenshotFiles(opts.OutputDir)
	if err != nil {
		return &CaptureResult{Logs: logs.String()}, err
	}

	hb.doneMsg(fmt.Sprintf("completed %d screenshots in %s", len(files), hb.elapsed()))
	hb.stop()

	return &CaptureResult{
		Files:      files,
		Logs:       logs.String(),
		ColorSpace: csInfo,
	}, nil
}

func (e *scriptEngine) DetectColorSpace(ctx context.Context, sourcePath string) (*ColorSpaceInfo, error) {
	return DetectColorSpace(ctx, sourcePath)
}

func (e *scriptEngine) CompressIfNeeded(ctx context.Context, path string, threshold int64, strategy string) (*CompressionResult, error) {
	return CompressIfNeeded(ctx, path, threshold, strategy)
}

func (e *scriptEngine) resolveScript(variant string) (string, error) {
	scriptName := "screenshots.sh"
	if variant == VariantJPG {
		scriptName = "screenshots_jpg.sh"
	}

	if envPath := strings.TrimSpace(os.Getenv("SCREENSHOT_SCRIPT")); envPath != "" {
		info, err := os.Stat(envPath)
		if err != nil {
			return "", fmt.Errorf("SCREENSHOT_SCRIPT not found: %v", err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("SCREENSHOT_SCRIPT must point to a file")
		}
		return envPath, nil
	}

	candidate := filepath.Join(e.screenshotScriptDir, scriptName)
	info, err := os.Stat(candidate)
	if err == nil && !info.IsDir() {
		return candidate, nil
	}

	envKey := "SCREENSHOT_SCRIPT"
	return "", fmt.Errorf("%s not found in %s; rebuild the image or set %s to override", scriptName, e.screenshotScriptDir, envKey)
}

func (e *scriptEngine) subtitleModeArgs(mode string) []string {
	if mode == SubtitleModeOff {
		return []string{"-nosub"}
	}
	return nil
}

func (e *scriptEngine) generateTimestamps(ctx context.Context, sourcePath string, count int) ([]float64, error) {
	if count <= 0 {
		count = defaultScreenshotCount
	}

	ffprobe, err := system.ResolveBin("FFPROBE_BIN", "ffprobe")
	if err != nil {
		return nil, err
	}

	duration, err := probeDuration(ctx, ffprobe, sourcePath)
	if err != nil {
		return nil, err
	}

	return buildRandomSecondsFloat(duration, count), nil
}

func (e *scriptEngine) listScreenshotFiles(dir string) ([]ScreenshotFileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]ScreenshotFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
			fullPath := filepath.Join(dir, entry.Name())
			info, _ := entry.Info()
			files = append(files, ScreenshotFileInfo{
				Path: fullPath,
				Name: entry.Name(),
				Size: info.Size(),
			})
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no screenshots were generated")
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}

func probeDuration(ctx context.Context, ffprobe, path string) (float64, error) {
	stdout, stderr, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	if err != nil {
		return 0, fmt.Errorf("ffprobe duration probe failed: %s", system.BestErrorMessage(err, stderr, stdout))
	}

	duration := 0.0
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, err := fmt.Sscanf(line, "%f", &duration); err == nil && duration > 0 {
			return duration, nil
		}
	}

	return 0, fmt.Errorf("ffprobe returned unusable duration")
}

func buildRandomSecondsFloat(duration float64, count int) []float64 {
	start := 0.0
	end := duration
	if duration > 120 {
		margin := duration * 0.08
		if margin < 15 {
			margin = 15
		}
		if margin > 300 {
			margin = 300
		}
		start = margin
		end = duration - margin
		if end <= start {
			start = 0
			end = duration
		}
	}

	rng := newRandom()
	step := (end - start) / float64(count)
	if step <= 0 {
		step = duration / float64(count+1)
	}

	values := make([]float64, 0, count)
	used := make(map[int]struct{}, count)
	for index := 0; index < count; index++ {
		segmentStart := start + step*float64(index)
		segmentEnd := segmentStart + step
		if index == count-1 || segmentEnd > end {
			segmentEnd = end
		}
		if segmentEnd <= segmentStart {
			segmentEnd = segmentStart + 1
		}

		value := segmentStart + rng.Float64()*(segmentEnd-segmentStart)
		if value < 0 {
			value = 0
		}
		if value >= duration {
			value = duration - 1
			if value < 0 {
				value = 0
			}
		}
		intVal := int(value)
		for try := 0; try < 8; try++ {
			if _, exists := used[intVal]; !exists {
				break
			}
			intVal++
		}
		used[intVal] = struct{}{}
		values = append(values, float64(intVal))
	}

	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values
}

func splitTimeline(aligned float64, coarseBack int) (int, float64) {
	if coarseBack < 0 {
		coarseBack = 0
	}
	coarseSecond := int(math.Max(math.Floor(aligned)-float64(coarseBack), 0))
	fineSecond := aligned - float64(coarseSecond)
	return coarseSecond, fineSecond
}

func formatTimestamp(totalSeconds int) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func newRandom() *randWrapper {
	return &randWrapper{seed: time.Now().UnixNano()}
}

type randWrapper struct {
	seed    int64
	counter int64
}

func (r *randWrapper) Float64() float64 {
	r.counter++
	val := float64((r.seed ^ (r.counter * 6364136223846793005)) & 0x7FFFFFFFFFFFFFFF)
	return val / 9223372036854775808.0
}