package screenshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mediainfo/internal/screenshot/delivery"
	"mediainfo/internal/screenshot/engine"
	"mediainfo/internal/screenshot/hosting"
)

const (
	ModeZip   = "zip"
	ModeLinks = "links"

	VariantPNG = "png"
	VariantJPG = "jpg"

	SubtitleModeAuto = "auto"
	SubtitleModeOff  = "off"
)

type ScriptResult struct {
	Files []ScreenshotFileInfo
	Logs  string
}

type UploadResult struct {
	Output string
	Logs   string
}

var defaultEngine = engine.New()

func NormalizeMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ModeLinks:
		return ModeLinks
	default:
		return ModeZip
	}
}

func NormalizeVariant(raw string) string {
	return engine.NormalizeVariant(raw)
}

func NormalizeSubtitleMode(raw string) string {
	return engine.NormalizeSubtitleMode(raw)
}

func NormalizeCount(raw string) int {
	return engine.NormalizeCount(raw)
}

func RunScript(ctx context.Context, inputPath, outputDir, variant, subtitleMode string, count int) ([]ScreenshotFileInfo, error) {
	result, err := RunScriptWithLogs(ctx, inputPath, outputDir, variant, subtitleMode, count)
	if err != nil {
		return nil, err
	}
	return result.Files, nil
}

func RunScriptWithLogs(ctx context.Context, inputPath, outputDir, variant, subtitleMode string, count int, logHandlers ...LogHandler) (ScriptResult, error) {
	var logs strings.Builder
	onLog := func(msg string) {
		logs.WriteString(msg)
		logs.WriteString("\n")
		for _, h := range logHandlers {
			if h != nil {
				h(msg)
			}
		}
	}

	result, err := runEngineScreenshotsWithLiveLogs(ctx, inputPath, outputDir, variant, subtitleMode, count, onLog)
	if err != nil {
		if len(result.Files) > 0 {
			files := make([]ScreenshotFileInfo, 0, len(result.Files))
			for _, path := range result.Files {
				info, _ := os.Stat(path)
				name := filepath.Base(path)
				var size int64
				if info != nil {
					size = info.Size()
				}
				files = append(files, ScreenshotFileInfo{Path: path, Name: name, Size: size})
			}
			return ScriptResult{Files: files, Logs: result.Logs}, err
		}
		return ScriptResult{}, err
	}
	files := make([]ScreenshotFileInfo, 0, len(result.Files))
	for _, path := range result.Files {
		info, _ := os.Stat(path)
		name := filepath.Base(path)
		var size int64
		if info != nil {
			size = info.Size()
		}
		files = append(files, ScreenshotFileInfo{
			Path: path,
			Name: name,
			Size: size,
		})
	}
	return ScriptResult{Files: files, Logs: result.Logs}, nil
}

func RunUpload(ctx context.Context, inputPath, outputDir, variant, subtitleMode, hostName string, count int) (string, error) {
	result, err := RunUploadWithLogs(ctx, inputPath, outputDir, variant, subtitleMode, hostName, count)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

func RunUploadWithLogs(ctx context.Context, inputPath, outputDir, variant, subtitleMode, hostName string, count int, logHandlers ...LogHandler) (UploadResult, error) {
	screenshotResult, err := RunScriptWithLogs(ctx, inputPath, outputDir, variant, subtitleMode, count, logHandlers...)
	if err != nil {
		return UploadResult{Logs: screenshotResult.Logs}, err
	}

	imagePaths := make([]string, 0, len(screenshotResult.Files))
	for _, f := range screenshotResult.Files {
		imagePaths = append(imagePaths, f.Path)
	}

	if len(imagePaths) == 0 {
		return UploadResult{Logs: screenshotResult.Logs}, errors.New("no screenshots were generated")
	}

	hostManager := hosting.NewManager()
	hostManager.Register(hosting.NewPixhost())
	hostManager.Register(hosting.NewFreeimage())
	hostManager.Register(hosting.NewLocal())
	hostManager.SetDefault("pixhost")

	host := hostManager.Get(hostName)
	if host == nil {
		return UploadResult{Logs: screenshotResult.Logs}, errors.New("unknown image host: " + hostName)
	}

	var logs strings.Builder
	logHandler := func(msg string) {
		logs.WriteString(msg)
		logs.WriteString("\n")
		for _, h := range logHandlers {
			if h != nil {
				h(msg)
			}
		}
	}

	links, err := host.Upload(ctx, imagePaths, logHandler)

	logStr := strings.TrimSpace(screenshotResult.Logs)
	if logs.Len() > 0 {
		if logStr != "" {
			logStr += "\n\n"
		}
		logStr += strings.TrimSpace(logs.String())
	}

	if err != nil {
		return UploadResult{Logs: logStr}, err
	}

	if len(links) == 0 {
		return UploadResult{Logs: logStr}, errors.New("host upload completed but returned no links")
	}

	output := strings.Join(links, "\n")
	return UploadResult{Output: output, Logs: logStr}, nil
}

func RunUploadFromDir(ctx context.Context, tempDir, hostName string, logHandlers ...LogHandler) (UploadResult, error) {
	imagePaths, err := delivery.ListImageFiles(tempDir)
	if err != nil {
		return UploadResult{}, err
	}
	if len(imagePaths) == 0 {
		return UploadResult{}, errors.New("no screenshots found in temp directory")
	}

	hostManager := hosting.NewManager()
	hostManager.Register(hosting.NewPixhost())
	hostManager.Register(hosting.NewFreeimage())
	hostManager.Register(hosting.NewLocal())
	hostManager.SetDefault("pixhost")

	host := hostManager.Get(hostName)
	if host == nil {
		return UploadResult{}, errors.New("unknown image host: " + hostName)
	}

	var logs strings.Builder
	logHandler := func(msg string) {
		logs.WriteString(msg)
		logs.WriteString("\n")
		for _, h := range logHandlers {
			if h != nil {
				h(msg)
			}
		}
	}

	links, err := host.Upload(ctx, imagePaths, logHandler)

	logStr := strings.TrimSpace(logs.String())
	if err != nil {
		return UploadResult{Logs: logStr}, err
	}

	if len(links) == 0 {
		return UploadResult{Logs: logStr}, errors.New("host upload completed but returned no links")
	}

	output := strings.Join(links, "\n")
	return UploadResult{Output: output, Logs: logStr}, nil
}

func RunEngineCapture(ctx context.Context, inputPath, outputDir, variant, subtitleMode string, count int) (*engine.CaptureResult, error) {
	return defaultEngine.Capture(ctx, engine.CaptureOptions{
		SourcePath:   inputPath,
		OutputDir:    outputDir,
		Variant:      variant,
		SubtitleMode: subtitleMode,
		Count:        count,
	})
}

func DetectColorSpace(ctx context.Context, sourcePath string) (*engine.ColorSpaceInfo, error) {
	return defaultEngine.DetectColorSpace(ctx, sourcePath)
}

func CompressScreenshot(ctx context.Context, path string, threshold int64, strategy string) (*engine.CompressionResult, error) {
	return defaultEngine.CompressIfNeeded(ctx, path, threshold, strategy)
}

func CompressAllScreenshots(ctx context.Context, files []string, threshold int64, strategy string) []*engine.CompressionResult {
	results := make([]*engine.CompressionResult, 0, len(files))
	for _, f := range files {
		result, err := defaultEngine.CompressIfNeeded(ctx, f, threshold, strategy)
		if err == nil && result.Compressed {
			results = append(results, result)
		}
	}
	return results
}

func listScreenshotFiles(dir string) ([]ScreenshotFileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]ScreenshotFileInfo, 0, 16)
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
		return nil, errors.New("no screenshots were generated")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
}
