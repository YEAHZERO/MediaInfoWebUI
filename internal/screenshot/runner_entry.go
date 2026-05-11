package screenshot

import (
	"context"
	"time"

	screenshotdvdinfo "mediainfo/internal/screenshot/dvdinfo"
	screenshotprogress "mediainfo/internal/screenshot/progress"
	screenshotruntime "mediainfo/internal/screenshot/runtime"
	screenshotsource "mediainfo/internal/screenshot/source"
	screenshottimestamps "mediainfo/internal/screenshot/timestamps"
)

type resolvedScreenshotSources struct {
	sourcePath       string
	dvdMediaInfoPath string
	cleanup          func()
}

type LogHandler = screenshotruntime.LineHandler
type ScreenshotsResult = screenshotruntime.ScreenshotsResult

func runEngineScreenshotsWithLiveLogs(ctx context.Context, inputPath, outputDir, variant, subtitleMode string, count int, onLog LogHandler) (ScreenshotsResult, error) {
	sources := resolveScreenshotSources(ctx, inputPath, onLog)

	timestamps, err := generateScreenshotTimestamps(ctx, sources.sourcePath, count, onLog)
	if err != nil {
		return ScreenshotsResult{}, err
	}

	return runScreenshotsFromSource(ctx, sources.sourcePath, sources.dvdMediaInfoPath, outputDir, variant, subtitleMode, timestamps, onLog)
}

func resolveScreenshotSources(ctx context.Context, inputPath string, onLog LogHandler) resolvedScreenshotSources {
	screenshotprogress.EmitStepLog(onLog, "启动", 1, 3, "正在解析截图输入源。")

	return resolvedScreenshotSources{
		sourcePath:       inputPath,
		dvdMediaInfoPath: "",
		cleanup:          func() {},
	}
}

func generateScreenshotTimestamps(ctx context.Context, sourcePath string, count int, onLog LogHandler) ([]float64, error) {
	detail := "正在估算影片时长并生成随机截图时间点。"
	screenshotprogress.EmitStepLog(onLog, "启动", 3, 3, detail)
	stopHeartbeat := screenshotprogress.StartHeartbeat(ctx, func(elapsed time.Duration) {
		screenshotprogress.EmitPercentLog(onLog, "启动", screenshotprogress.SubtitleHeartbeatStepPercent(elapsed), screenshotprogress.SubtitleHeartbeatDetail(detail, elapsed))
	})
	timestamps := generateScreenshotTimestampsFromSource(ctx, sourcePath, count)
	stopHeartbeat()
	return timestamps, nil
}

func runScreenshotsFromSource(ctx context.Context, sourcePath, dvdMediaInfoPath, outputDir, variant, subtitleMode string, timestamps []float64, onLog LogHandler) (ScreenshotsResult, error) {
	runner := newScreenshotRunner(ctx, sourcePath, dvdMediaInfoPath, outputDir, variant, subtitleMode, onLog)
	defer runner.cleanupTemporarySubtitleResources()

	runner.logRuntimeBootstrap()
	if err := runner.init(timestamps); err != nil {
		return ScreenshotsResult{Logs: runner.logs()}, err
	}

	files, err := runner.run()
	if err != nil {
		return ScreenshotsResult{Logs: runner.logs()}, err
	}
	return ScreenshotsResult{
		Files:         files,
		Logs:          runner.logs(),
		LossyPNGFiles: runner.lossyPNGFileList(),
	}, nil
}

func newScreenshotRunner(ctx context.Context, sourcePath, dvdMediaInfoPath, outputDir, variant, subtitleMode string, onLog LogHandler) *screenshotRunner {
	normalizedVariant := NormalizeVariant(variant)
	normalizedSubtitleMode := NormalizeSubtitleMode(subtitleMode)
	return &screenshotRunner{
		ctx:              ctx,
		sourcePath:       sourcePath,
		dvdMediaInfoPath: dvdMediaInfoPath,
		outputDir:        outputDir,
		variant:          normalizedVariant,
		subtitleMode:     normalizedSubtitleMode,
		settings:         screenshotruntime.VariantSettingsFor(normalizedVariant),
		subtitle: screenshotruntime.SubtitleSelection{
			Mode: "none",
		},
		logger:        screenshotruntime.NewLogger(onLog),
		lossyPNGFiles: make(map[string]struct{}),
	}
}

func (r *screenshotRunner) logRuntimeBootstrap() {
	r.logf("[信息] 已切换为 Go 截图引擎。")
	if !screenshotsource.LooksLikeDVDSource(r.sourcePath) {
		return
	}

	selectedVOBPath := screenshotdvdinfo.ResolveVOBPath(r.sourcePath, r.dvdMediaInfoPath)
	selectedIFOPath := screenshotdvdinfo.ResolveProbePath(r.sourcePath, r.dvdMediaInfoPath)
	r.logf("[信息] DVD 已选片段：VOB=%s | IFO=%s",
		screenshottimestamps.DisplayProbeValue(selectedVOBPath),
		screenshottimestamps.DisplayProbeValue(selectedIFOPath),
	)
}