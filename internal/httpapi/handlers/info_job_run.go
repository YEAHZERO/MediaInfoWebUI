package handlers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"mediainfo/internal/config"
	"mediainfo/internal/filelogger"
	"mediainfo/internal/media"
	"mediainfo/internal/system"
)

func (j *infoJob) run() {
	defer func() {
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

	switch j.kind {
	case infoKindMediaInfo:
		bin, err := system.ResolveBin("MEDIAINFO_BIN", "mediainfo")
		if err != nil {
			filelogger.Log(filelogger.MediaInfo, "[%s] 未找到可执行文件: %s", j.id, err.Error())
			j.logger.Logf("[mediainfo] 未找到可执行文件: %s", err.Error())
			j.fail(err)
			return
		}
		filelogger.Log(filelogger.MediaInfo, "[%s] 输入路径: %s", j.id, j.inputPath)
		filelogger.Log(filelogger.MediaInfo, "[%s] 使用命令: %s", j.id, bin)
		j.logger.Logf("[mediainfo] 输入路径: %s", j.inputPath)
		j.logger.Logf("[mediainfo] 使用命令: %s", bin)

		output, err := runMediaInfoJob(ctx, j.inputPath, j.logger, bin)
		if err != nil {
			filelogger.Log(filelogger.MediaInfo, "[%s] 失败: %s", j.id, err.Error())
			j.fail(err)
			return
		}
		filelogger.Log(filelogger.MediaInfo, "[%s] 完成", j.id)
		j.succeed(output)
	case infoKindBDInfo:
		filelogger.Log(filelogger.BDInfo, "[%s] 输入路径: %s", j.id, j.inputPath)
		j.logger.Logf("[bdinfo] 输入路径: %s", j.inputPath)
		output, err := runBDInfoJob(ctx, j.inputPath, j.bdinfoMode, j.logger)
		if err != nil {
			filelogger.Log(filelogger.BDInfo, "[%s] 失败: %s", j.id, err.Error())
			j.fail(err)
			return
		}
		filelogger.Log(filelogger.BDInfo, "[%s] 完成", j.id)
		j.succeed(output)
	default:
		j.fail(errors.New("unsupported info job kind"))
	}
}

func runMediaInfoJob(ctx context.Context, path string, logger *infoLogger, bin string) (string, error) {
	candidates, sourceCleanup, err := media.ResolveMediaInfoCandidates(ctx, path, media.MediaInfoCandidateLimit)
	if err != nil {
		logger.Logf("[mediainfo] 解析候选源失败: %s", err.Error())
		return "", err
	}
	defer sourceCleanup()
	logger.Logf("[mediainfo] 候选源数量: %d", len(candidates))

	var lastErr string
	for idx, sourcePath := range candidates {
		sourceDir := filepath.Dir(sourcePath)
		sourceName := filepath.Base(sourcePath)
		logger.Logf("[mediainfo] 尝试 %d/%d: %s", idx+1, len(candidates), sourcePath)

		stdout, stderr, err := system.RunCommandInDirLive(ctx, sourceDir, bin, logger.CommandOutput("mediainfo"), sourceName)
		if err != nil {
			lastErr = system.BestErrorMessage(err, stderr, stdout)
			logger.LogMultiline("[mediainfo][error] ", lastErr)
			continue
		}

		output := system.CombineCommandOutput(stdout, stderr)
		if output == "" {
			lastErr = fmt.Sprintf("mediainfo returned empty output for: %s", sourcePath)
			logger.Logf("[mediainfo] 返回空输出: %s", sourcePath)
			continue
		}

		logger.Logf("[mediainfo] 完成: %s", sourcePath)
		return output, nil
	}

	if lastErr == "" {
		lastErr = "mediainfo returned empty output"
	}
	return "", fmt.Errorf("%s", lastErr)
}

func runBDInfoJob(ctx context.Context, path, mode string, logger *infoLogger) (string, error) {
	bin, err := system.ResolveBin("BDINFO_BIN", "bdinfo")
	if err != nil {
		logger.Logf("[bdinfo] 未找到可执行文件: %s", err.Error())
		return "", err
	}

	bdPath, bdCleanup, err := media.ResolveBDInfoSource(ctx, path)
	if err != nil {
		logger.Logf("[bdinfo] 解析路径失败: %s", err.Error())
		return "", err
	}
	defer bdCleanup()

	logger.Logf("[bdinfo] 使用命令: %s", bin)
	logger.Logf("[bdinfo] 实际检测路径: %s", bdPath)

	stdout, stderr, err := system.RunCommandLive(ctx, bin, logger.CommandOutput("bdinfo"), bdPath)
	if err != nil {
		logger.LogMultiline("[bdinfo][error] ", system.BestErrorMessage(err, stderr, stdout))
		return "", err
	}

	output := system.CombineCommandOutput(stdout, stderr)
	if shouldExtractBDInfoCode(mode) {
		logger.Logf("[bdinfo] 输出模式: 精简报告")
		output = extractBDInfoCodeBlock(output)
	} else {
		logger.Logf("[bdinfo] 输出模式: 完整报告")
	}

	logger.Logf("[bdinfo] 完成")
	return output, nil
}