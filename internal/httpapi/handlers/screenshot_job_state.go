package handlers

import (
	"context"
	"errors"
	"time"

	"mediainfo/internal/httpapi/transport"
)

func (j *screenshotJob) snapshot() transport.ScreenshotJobResponse {
	j.mu.RLock()
	count := j.count
	response := transport.ScreenshotJobResponse{
		OK:              true,
		JobID:           j.id,
		Status:          j.status,
		Mode:            j.mode,
		Output:          j.output,
		DownloadURL:     j.downloadURL,
		LinkItems:       append([]transport.ImageLinkItem(nil), j.linkItems...),
		Error:           j.errMessage,
		PNGLossyFiles:   append([]string(nil), j.pngLossyFiles...),
		PNGLossyIndexes: append([]int(nil), j.pngLossyIndexes...),
	}
	logger := j.logger
	j.mu.RUnlock()

	var entries []transport.LogEntry
	if logger != nil {
		response.Logs = logger.String()
		entries = logger.Entries()
		response.LogEntries = entries
	}
	response.Progress = buildScreenshotTaskProgress(response.Mode, response.Status, count, response.LogEntries)
	return response
}

func (j *screenshotJob) expired(now time.Time) bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.completedAt.IsZero() {
		return false
	}
	return now.Sub(j.completedAt) > screenshotJobTTL
}

func (j *screenshotJob) beginRun() bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.status != screenshotJobStatusPending {
		return false
	}
	if j.cancelRequested || errors.Is(j.taskContext.Err(), context.Canceled) {
		return false
	}

	j.status = screenshotJobStatusRunning
	j.updatedAt = time.Now()
	return true
}

func (j *screenshotJob) requestCancel() {
	var cancel context.CancelFunc

	j.mu.Lock()
	switch j.status {
	case screenshotJobStatusSucceeded, screenshotJobStatusFailed, screenshotJobStatusCanceled:
		j.mu.Unlock()
		return
	case screenshotJobStatusCanceling:
		j.mu.Unlock()
		return
	default:
		j.cancelRequested = true
		j.status = screenshotJobStatusCanceling
		j.errMessage = "任务取消中。"
		j.updatedAt = time.Now()
		cancel = j.cancel
		j.mu.Unlock()
	}

	if cancel != nil {
		cancel()
	}
}

func (j *screenshotJob) succeed(output, downloadURL string, linkItems []transport.ImageLinkItem, pngLossyFiles []string, pngLossyIndexes []int) {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	if j.cancelRequested || errors.Is(j.taskContext.Err(), context.Canceled) {
		j.status = screenshotJobStatusCanceled
		j.output = ""
		j.downloadURL = ""
		j.linkItems = nil
		j.pngLossyFiles = nil
		j.pngLossyIndexes = nil
		j.errMessage = "任务已取消。"
		j.updatedAt = now
		j.completedAt = now
		return
	}

	j.status = screenshotJobStatusSucceeded
	j.output = output
	j.downloadURL = downloadURL
	j.linkItems = append([]transport.ImageLinkItem(nil), linkItems...)
	j.pngLossyFiles = append([]string(nil), pngLossyFiles...)
	j.pngLossyIndexes = append([]int(nil), pngLossyIndexes...)
	j.errMessage = ""
	j.updatedAt = now
	j.completedAt = now
}

func (j *screenshotJob) fail(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	if j.cancelRequested || isScreenshotJobCanceledError(err) || errors.Is(j.taskContext.Err(), context.Canceled) {
		j.status = screenshotJobStatusCanceled
		j.output = ""
		j.downloadURL = ""
		j.linkItems = nil
		j.pngLossyFiles = nil
		j.pngLossyIndexes = nil
		j.errMessage = "任务已取消。"
		j.updatedAt = now
		j.completedAt = now
		return
	}

	j.status = screenshotJobStatusFailed
	j.output = ""
	j.downloadURL = ""
	j.linkItems = nil
	j.pngLossyFiles = nil
	j.pngLossyIndexes = nil
	if err != nil {
		j.errMessage = err.Error()
	} else {
		j.errMessage = "job failed"
	}
	j.updatedAt = now
	j.completedAt = now
}

func (j *screenshotJob) finishCanceled() {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.status == screenshotJobStatusSucceeded || j.status == screenshotJobStatusFailed || j.status == screenshotJobStatusCanceled {
		return
	}

	now := time.Now()
	j.status = screenshotJobStatusCanceled
	j.output = ""
	j.downloadURL = ""
	j.linkItems = nil
	j.pngLossyFiles = nil
	j.pngLossyIndexes = nil
	j.errMessage = "任务已取消。"
	j.updatedAt = now
	j.completedAt = now
}

func (j *screenshotJob) isCancellationRequested() bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.cancelRequested || errors.Is(j.taskContext.Err(), context.Canceled)
}

func isScreenshotJobCanceledError(err error) bool {
	return errors.Is(err, context.Canceled)
}