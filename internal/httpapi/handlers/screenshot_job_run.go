package handlers

import (
	"context"
	"os"
	"strings"

	"mediainfo/internal/filelogger"
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

	filelogger.Log(filelogger.Screenshots, "[%s] 开始任务: path=%s mode=%s host=%s count=%d", j.id, j.inputPath, j.mode, j.host, j.count)

	if !j.beginRun() {
		filelogger.Log(filelogger.Screenshots, "[%s] 任务取消或重复", j.id)
		if j.isCancellationRequested() {
			j.finishCanceled()
		}
		return
	}

	ctx, cancel := context.WithCancel(j.taskContext)
	defer cancel()

	tempDir, err := createScreenshotTempDir("mediainfo-screenshot-job-*")
	if err != nil {
		filelogger.Log(filelogger.Screenshots, "[%s] 创建临时目录失败: %s", j.id, err.Error())
		j.fail(err)
		return
	}
	defer os.RemoveAll(tempDir)

	switch j.mode {
	case screenshot.ModeLinks:
		filelogger.Log(filelogger.Screenshots, "[%s] 图床模式: host=%s", j.id, j.host)
		result, err := screenshot.RunUploadWithLogs(ctx, j.inputPath, tempDir, j.variant, j.subtitleMode, j.host, j.count)
		if err != nil {
			filelogger.Log(filelogger.Screenshots, "[%s] 失败: %s", j.id, err.Error())
			if result.Logs != "" {
				j.logger.LogLine(result.Logs)
			}
			j.fail(err)
			return
		}
		filelogger.Log(filelogger.Screenshots, "[%s] 成功: 生成了 %d 个链接", j.id, len(parseUploadLinks(result.Output)))
		if result.Logs != "" {
			j.logger.LogLine(result.Logs)
		}
		linkItems := parseUploadLinks(result.Output)
		j.succeed(result.Output, "", linkItems, nil, nil)
	default:
		filelogger.Log(filelogger.Screenshots, "[%s] 下载模式", j.id)
		downloadURL, logs, _, err := prepareScreenshotZipDownload(ctx, j.inputPath, tempDir, j.variant, j.subtitleMode, j.count)
		if err != nil {
			filelogger.Log(filelogger.Screenshots, "[%s] 打包下载失败: %s", j.id, err.Error())
			if logs != "" {
				j.logger.LogLine(logs)
			}
			j.fail(err)
			return
		}
		filelogger.Log(filelogger.Screenshots, "[%s] 打包下载成功", j.id)
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