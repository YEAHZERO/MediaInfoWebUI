package screenshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mediainfo/internal/screenshot/engine"
	"mediainfo/internal/system"
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

func RunScriptWithLogs(ctx context.Context, inputPath, outputDir, variant, subtitleMode string, count int) (ScriptResult, error) {
	capResult, err := defaultEngine.Capture(ctx, engine.CaptureOptions{
		SourcePath:   inputPath,
		OutputDir:    outputDir,
		Variant:      variant,
		SubtitleMode: subtitleMode,
		Count:        count,
	})
	if err != nil {
		if capResult != nil {
			files := make([]ScreenshotFileInfo, 0, len(capResult.Files))
			for _, f := range capResult.Files {
				files = append(files, ScreenshotFileInfo{Path: f.Path, Name: f.Name, Size: f.Size})
			}
			return ScriptResult{Files: files, Logs: capResult.Logs}, err
		}
		return ScriptResult{}, err
	}
	files := make([]ScreenshotFileInfo, 0, len(capResult.Files))
	for _, f := range capResult.Files {
		files = append(files, ScreenshotFileInfo{
			Path: f.Path,
			Name: f.Name,
			Size: f.Size,
		})
	}
	return ScriptResult{Files: files, Logs: capResult.Logs}, nil
}

func RunUpload(ctx context.Context, inputPath, outputDir, variant, subtitleMode string, count int) (string, error) {
	result, err := RunUploadWithLogs(ctx, inputPath, outputDir, variant, subtitleMode, count)
	if err != nil {
		return "", err
	}
	return result.Output, nil
}

func RunUploadWithLogs(ctx context.Context, inputPath, outputDir, variant, subtitleMode string, count int) (UploadResult, error) {
	uploadScript, err := resolveUploadScript()
	if err != nil {
		return UploadResult{}, err
	}

	screenshotResult, err := RunScriptWithLogs(ctx, inputPath, outputDir, variant, subtitleMode, count)
	if err != nil {
		return UploadResult{Logs: screenshotResult.Logs}, err
	}

	stdout, stderr, err := system.RunCommand(ctx, "bash", uploadScript, outputDir)
	uploadLogs := system.CombineCommandOutput(stdout, stderr)
	logs := strings.TrimSpace(strings.Join(filterNonEmptyStrings(screenshotResult.Logs, uploadLogs), "\n\n"))
	if err != nil {
		return UploadResult{Logs: logs}, err
	}

	links := extractDirectLinks(stdout)
	if len(links) == 0 {
		output := strings.TrimSpace(stdout)
		if output == "" {
			output = strings.TrimSpace(stderr)
		}
		if output == "" {
			return UploadResult{Logs: logs}, errors.New("pixhost upload completed but returned no links")
		}
		return UploadResult{Output: output, Logs: logs}, nil
	}
	return UploadResult{Output: strings.Join(links, "\n"), Logs: logs}, nil
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

func resolveUploadScript() (string, error) {
	const scriptDir = "/usr/local/share/mediainfo/scripts"
	const name = "PixhostUpload.sh"

	if value := strings.TrimSpace(os.Getenv("SCREENSHOT_UPLOAD_SCRIPT")); value != "" {
		info, err := os.Stat(value)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", errors.New("SCREENSHOT_UPLOAD_SCRIPT must point to a file")
		}
		return value, nil
	}

	candidate := filepath.Join(scriptDir, name)
	info, err := os.Stat(candidate)
	if err == nil && !info.IsDir() {
		return candidate, nil
	}

	return "", errors.New("PixhostUpload.sh not found; set SCREENSHOT_UPLOAD_SCRIPT")
}

func extractDirectLinks(output string) []string {
	lines := strings.Split(output, "\n")
	links := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
			continue
		}
		if strings.ContainsAny(line, " []()<>\"") {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		links = append(links, line)
	}
	return links
}

func filterNonEmptyStrings(values ...string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
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
