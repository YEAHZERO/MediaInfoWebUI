package handlers

import (
	"context"
	"errors"
	"time"

	"mediainfo/internal/httpapi/transport"
)

func (j *infoJob) snapshot() transport.InfoJobResponse {
	j.mu.RLock()
	response := transport.InfoJobResponse{
		OK:     true,
		JobID:  j.id,
		Status: j.status,
		Kind:   j.kind,
		Output: j.output,
		Error:  j.errMessage,
	}
	logger := j.logger
	j.mu.RUnlock()

	var entries []transport.LogEntry
	if logger != nil {
		response.Logs = logger.String()
		entries = logger.Entries()
		response.LogEntries = entries
	}
	response.Progress = buildInfoTaskProgress(response.Kind, response.Status, entries)
	return response
}

func (j *infoJob) expired(now time.Time) bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if j.completedAt.IsZero() {
		return false
	}
	return now.Sub(j.completedAt) > infoJobTTL
}

func (j *infoJob) beginRun() bool {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.status != infoJobStatusPending {
		return false
	}
	if j.cancelRequested || errors.Is(j.taskContext.Err(), context.Canceled) {
		return false
	}

	j.status = infoJobStatusRunning
	j.updatedAt = time.Now()
	return true
}

func (j *infoJob) requestCancel() {
	var cancel context.CancelFunc

	j.mu.Lock()
	switch j.status {
	case infoJobStatusSucceeded, infoJobStatusFailed, infoJobStatusCanceled:
		j.mu.Unlock()
		return
	case infoJobStatusCanceling:
		j.mu.Unlock()
		return
	default:
		j.cancelRequested = true
		j.status = infoJobStatusCanceling
		j.errMessage = "任务取消中。"
		j.updatedAt = time.Now()
		cancel = j.cancel
		j.mu.Unlock()
	}

	if cancel != nil {
		cancel()
	}
}

func (j *infoJob) succeed(output string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	if j.cancelRequested || errors.Is(j.taskContext.Err(), context.Canceled) {
		j.status = infoJobStatusCanceled
		j.output = ""
		j.errMessage = "任务已取消。"
		j.updatedAt = now
		j.completedAt = now
		return
	}

	j.status = infoJobStatusSucceeded
	j.output = output
	j.errMessage = ""
	j.updatedAt = now
	j.completedAt = now
}

func (j *infoJob) fail(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	now := time.Now()
	if j.cancelRequested || isInfoJobCanceledError(err) || errors.Is(j.taskContext.Err(), context.Canceled) {
		j.status = infoJobStatusCanceled
		j.output = ""
		j.errMessage = "任务已取消。"
		j.updatedAt = now
		j.completedAt = now
		return
	}

	j.status = infoJobStatusFailed
	j.output = ""
	if err != nil {
		j.errMessage = err.Error()
	} else {
		j.errMessage = "job failed"
	}
	j.updatedAt = now
	j.completedAt = now
}

func (j *infoJob) finishCanceled() {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.status == infoJobStatusSucceeded || j.status == infoJobStatusFailed || j.status == infoJobStatusCanceled {
		return
	}

	now := time.Now()
	j.status = infoJobStatusCanceled
	j.output = ""
	j.errMessage = "任务已取消。"
	j.updatedAt = now
	j.completedAt = now
}

func (j *infoJob) isCancellationRequested() bool {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.cancelRequested || errors.Is(j.taskContext.Err(), context.Canceled)
}

func isInfoJobCanceledError(err error) bool {
	return errors.Is(err, context.Canceled)
}