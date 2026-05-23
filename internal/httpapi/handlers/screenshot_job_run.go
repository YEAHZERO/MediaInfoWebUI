package handlers

import (
	"context"
	"os"
	"strings"
	"time"

	"mediainfo/internal/filelogger"
	"mediainfo/internal/httpapi/transport"
	"mediainfo/internal/screenshot"
)

func (j *screenshotJob) run() {
	screenshotJobSem <- struct{}{}
	jobStart := time.Now()
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

	modeLabel := "下载"
	if j.mode == screenshot.ModeLinks {
		modeLabel = "图床"
	}
	variantLabel := "PNG"
	if j.variant == "jpg" {
		variantLabel = "JPG"
	}
	subtitleLabel := "自动"
	if j.subtitleMode == "off" {
		subtitleLabel = "关闭"
	} else if j.subtitleMode == "force" {
		subtitleLabel = "强制"
	}
	hostLabel := j.host
	if hostLabel == "" {
		hostLabel = "无"
	}

	filelogger.Log(filelogger.Screenshots, "[%s] 点击: 截图 | 路径=%s | 格式=%s | 字幕=%s | 数量=%d | 模式=%s | 图床=%s",
		j.id, j.inputPath, variantLabel, subtitleLabel, j.count, modeLabel, hostLabel)

	if !j.beginRun() {
		filelogger.Log(filelogger.Screenshots, "[%s] 跳过: 任务已取消或重复", j.id)
		if j.isCancellationRequested() {
			j.finishCanceled()
		}
		return
	}

	ctx, cancel := context.WithCancel(j.taskContext)
	defer cancel()

	tempDir, err := createScreenshotTempDir("mediainfo-screenshot-job-*")
	if err != nil {
		filelogger.Log(filelogger.Screenshots, "[%s] 失败: 创建临时目录 - %s | 耗时=%s", j.id, err.Error(), time.Since(jobStart))
		j.fail(err)
		return
	}
	defer os.RemoveAll(tempDir)

	switch j.mode {
	case screenshot.ModeLinks:
		filelogger.Log(filelogger.Screenshots, "[%s] 开始截图并上传至: %s", j.id, j.host)
		result, err := screenshot.RunUploadWithLogs(ctx, j.inputPath, tempDir, j.variant, j.subtitleMode, j.host, j.count, j.logger.LogLine)
		if err != nil {
			filelogger.Log(filelogger.Screenshots, "[%s] 失败: %s | 耗时=%s", j.id, err.Error(), time.Since(jobStart))
			if result.Logs != "" {
				filelogger.Log(filelogger.Screenshots, "[%s] 截图详情:\n%s", j.id, result.Logs)
			}
			if j.logger != nil {
				j.logger.LogLine(result.Logs)
			}
			j.fail(err)
			return
		}
		links := parseUploadLinks(result.Output)
		filelogger.Log(filelogger.Screenshots, "[%s] 成功: 截图 %d 张 | 上传 %d 个链接 | 耗时=%s", j.id, j.count, len(links), time.Since(jobStart))
		if result.Logs != "" && j.logger != nil {
			j.logger.LogLine(result.Logs)
		}
		var downloadURL string
		zipBytes, _, zipErr := zipScreenshotsFromDir(tempDir)
		if zipErr == nil {
			token, saveErr := screenshot.SavePreparedDownload(zipBytes)
			if saveErr == nil {
				downloadURL = "/api/screenshots?token=" + token
			}
		}
		linkItems := parseUploadLinks(result.Output)
		j.succeed(result.Output, downloadURL, linkItems, nil, nil)
	default:
		filelogger.Log(filelogger.Screenshots, "[%s] 开始截图并打包下载", j.id)
		downloadURL, logs, _, err := prepareScreenshotZipDownload(ctx, j.inputPath, tempDir, j.variant, j.subtitleMode, j.count, j.logger.LogLine)
		if err != nil {
			filelogger.Log(filelogger.Screenshots, "[%s] 失败: %s | 耗时=%s", j.id, err.Error(), time.Since(jobStart))
			if logs != "" {
				filelogger.Log(filelogger.Screenshots, "[%s] 截图详情:\n%s", j.id, logs)
			}
			if logs != "" && j.logger != nil {
				j.logger.LogLine(logs)
			}
			j.fail(err)
			return
		}
		filelogger.Log(filelogger.Screenshots, "[%s] 成功: 截图 %d 张 | 打包完成 | 耗时=%s", j.id, j.count, time.Since(jobStart))
		if logs != "" && j.logger != nil {
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