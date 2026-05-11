package handlers

import (
	"context"
	"os"
	"strings"
	"time"

	"mediainfo/internal/config"
	"mediainfo/internal/httpapi/transport"
	"mediainfo/internal/screenshot"
)

func (j *screenshotJob) run() {
	screenshotJobSem <- struct{}{}
	defer func() {
		<-screenshotJobSem
		if j.cancel != nil {
			j.cancel()
		}
		if j.cleanup != nil {
			j.cleanup()
		}
		if j.logger != nil {
			j.logger.Close()
		}
	}()

	if !j.beginRun() {
		if j.isCancellationRequested() {
			j.finishCanceled()
		}
		return
	}

	ctx, cancel := context.WithTimeout(j.taskContext, config.RequestTimeout)
	defer cancel()

	tempDir, err := createScreenshotTempDir("mediainfo-screenshot-job-*")
	if err != nil {
		j.fail(err)
		return
	}
	defer os.RemoveAll(tempDir)

	switch j.mode {
	case screenshot.ModeLinks:
		result, err := screenshot.RunUploadWithLogs(ctx, j.inputPath, tempDir, j.variant, j.subtitleMode, j.count)
		if err != nil {
			if result.Logs != "" {
				j.logger.LogLine(result.Logs)
			}
			j.fail(err)
			return
		}
		if result.Logs != "" {
			j.logger.LogLine(result.Logs)
		}
		linkItems := parseUploadLinks(result.Output)
		j.succeed(result.Output, "", linkItems, nil, nil)
	default:
		downloadURL, logs, _, err := prepareScreenshotZipDownload(ctx, j.inputPath, tempDir, j.variant, j.subtitleMode, j.count)
		if err != nil {
			if logs != "" {
				j.logger.LogLine(logs)
			}
			j.fail(err)
			return
		}
		if logs != "" {
			j.logger.LogLine(logs)
		}
		j.succeed("", downloadURL, nil, nil, nil)
	}
}

func parseUploadLinks(output string) []transport.ImageLinkItem {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	items := make([]transport.ImageLinkItem, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "http://") && !strings.HasPrefix(line, "https://") {
			continue
		}
		items = append(items, transport.ImageLinkItem{
			URL:      line,
			Filename: "",
			Size:     0,
		})
	}
	return items
}

func createScreenshotTempDir(pattern string) (string, error) {
	return os.MkdirTemp("", pattern)
}

func (j *screenshotJob) appendLinkItem(item interface{}) {
}

func contextWithCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}

func _() {
	_ = time.Second
	_ = config.RequestTimeout
}