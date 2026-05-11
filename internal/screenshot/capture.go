package screenshot

import (
	"context"
	"errors"
	"fmt"
	"time"

	screenshotprogress "mediainfo/internal/screenshot/progress"
	screenshotruntime "mediainfo/internal/screenshot/runtime"
	screenshotsource "mediainfo/internal/screenshot/source"
	screenshottimestamps "mediainfo/internal/screenshot/timestamps"
)

type RunScreenshotsOptions struct {
	Ctx          context.Context
	SourcePath   string
	DVDMediaInfo string
	OutputDir    string
	Variant      string
	SubtitleMode string
	Count        int
	OnLog        screenshotruntime.LineHandler
}

func RunScreenshotsLive(opts RunScreenshotsOptions) (screenshotruntime.ScreenshotsResult, error) {
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}

	sources := resolveScreenshotSources(opts.Ctx, opts.SourcePath, opts.OnLog)
	defer sources.cleanup()

	timestamps, err := generateScreenshotTimestamps(opts.Ctx, sources.sourcePath, opts.Count, opts.OnLog)
	if err != nil {
		return screenshotruntime.ScreenshotsResult{}, err
	}

	result, err := runScreenshotsFromSource(opts.Ctx, sources.sourcePath, sources.dvdMediaInfoPath, opts.OutputDir, opts.Variant, opts.SubtitleMode, timestamps, opts.OnLog)
	if err != nil {
		return result, err
	}

	return result, nil
}

func CaptureScreenshots(ctx context.Context, sourcePath, outputDir, variant, subtitleMode string, count int, onLog screenshotruntime.LineHandler) ([]string, error) {
	if onLog == nil {
		onLog = func(string) {}
	}
	screenshotprogress.EmitStepLog(onLog, "开始", 1, 4, "正在启动截图流程。")

	sources := resolveScreenshotSources(ctx, sourcePath, onLog)
	defer sources.cleanup()

	timestamps, err := generateScreenshotTimestamps(ctx, sources.sourcePath, count, onLog)
	if err != nil {
		return nil, err
	}

	screenshotprogress.EmitStepLog(onLog, "开始", 2, 4, "正在初始化截图运行器。")
	runner := newScreenshotRunner(ctx, sources.sourcePath, sources.dvdMediaInfoPath, outputDir, variant, subtitleMode, onLog)
	defer runner.cleanupTemporarySubtitleResources()

	runner.logRuntimeBootstrap()
	if err := runner.init(timestamps); err != nil {
		return nil, fmt.Errorf("截图初始化失败：%w\n%s", err, runner.logs())
	}

	screenshotprogress.EmitStepLog(onLog, "运行", 3, 4, "正在渲染截图。")
	files, err := runner.run()
	if err != nil {
		return nil, fmt.Errorf("截图运行失败：%w\n%s", err, runner.logs())
	}

	screenshotprogress.EmitStepLog(onLog, "完成", 4, 4, fmt.Sprintf("截图流程完成，共生成 %d 张截图。", len(files)))
	return files, nil
}

func RunScreenshotsWithRetry(ctx context.Context, sourcePath, outputDir, variant, subtitleMode string, count int, onLog screenshotruntime.LineHandler, maxRetries int) ([]string, error) {
	if onLog == nil {
		onLog = func(string) {}
	}
	if maxRetries <= 0 {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			screenshotprogress.EmitStepLog(onLog, "重试", attempt-1, maxRetries-1, fmt.Sprintf("第 %d 次重试（共 %d 次）...", attempt-1, maxRetries-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt-1) * 2 * time.Second):
			}
		}

		files, err := CaptureScreenshots(ctx, sourcePath, outputDir, variant, subtitleMode, count, onLog)
		if err == nil {
			return files, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("截图在 %d 次尝试后均失败：%w", maxRetries, lastErr)
}

func resolveSource(inputPath string) (string, string, func()) {
	dvdMediaInfoPath := ""
	if screenshotsource.LooksLikeDVDSource(inputPath) {
		dvdMediaInfoPath = inputPath
	}
	return inputPath, dvdMediaInfoPath, func() {}
}

func probeSourceDuration(ctx context.Context, sourcePath, ffprobeBin string) (float64, error) {
	if ffprobeBin == "" {
		return 0, errors.New("ffprobe binary not available")
	}
	return screenshottimestamps.ProbeDuration(ctx, ffprobeBin, sourcePath)
}