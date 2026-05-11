package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"mediainfo/internal/httpapi/transport"
	"mediainfo/internal/system"
)

type infoLogger struct {
	mu    sync.Mutex
	lines []timedLogLine
}

type timedLogLine struct {
	timestamp time.Time
	message   string
}

func newInfoLogger() *infoLogger {
	return &infoLogger{
		lines: make([]timedLogLine, 0, 32),
	}
}

func (l *infoLogger) Logf(format string, args ...any) {
	if l == nil {
		return
	}
	now := time.Now()
	line := fmt.Sprintf(format, args...)
	l.mu.Lock()
	l.lines = append(l.lines, timedLogLine{
		timestamp: now,
		message:   line,
	})
	l.mu.Unlock()
}

func (l *infoLogger) LogLine(line string) {
	if l == nil {
		return
	}
	l.Logf("%s", line)
}

func (l *infoLogger) LogMultiline(prefix, text string) {
	if l == nil {
		return
	}
	for _, line := range splitLogLines(text) {
		if prefix == "" {
			l.Logf("%s", line)
			continue
		}
		l.Logf("%s%s", prefix, line)
	}
}

func (l *infoLogger) CommandOutput(scope string) system.OutputLineHandler {
	return func(stream, line string) {
		l.Logf("[%s][%s] %s", scope, stream, line)
	}
}

func (l *infoLogger) String() string {
	if l == nil {
		return ""
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.lines) == 0 {
		return ""
	}

	formatted := make([]string, 0, len(l.lines))
	for _, line := range l.lines {
		if line.timestamp.IsZero() {
			formatted = append(formatted, line.message)
			continue
		}
		formatted = append(formatted, fmt.Sprintf("[%s] %s", line.timestamp.Format("15:04:05"), line.message))
	}
	return strings.Join(formatted, "\n")
}

func (l *infoLogger) Entries() []transport.LogEntry {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.lines) == 0 {
		return nil
	}

	entries := make([]transport.LogEntry, 0, len(l.lines))
	for _, line := range l.lines {
		entry := transport.LogEntry{
			Message: line.message,
		}
		if !line.timestamp.IsZero() {
			entry.Timestamp = line.timestamp.UTC().Format(time.RFC3339Nano)
		}
		entries = append(entries, entry)
	}
	return entries
}

func (l *infoLogger) Close() {
}

func writeInfoError(w http.ResponseWriter, status int, message string, logger *infoLogger) {
	transport.WriteJSON(w, status, transport.InfoResponse{
		OK:         false,
		Error:      message,
		Logs:       logger.String(),
		LogEntries: logger.Entries(),
	})
}

func splitLogLines(text string) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if normalized == "" {
		return []string{""}
	}
	return strings.Split(normalized, "\n")
}

func pickRealtimeLogs(logger *infoLogger, fallback string) string {
	if logger == nil {
		return fallback
	}
	if logs := logger.String(); strings.TrimSpace(logs) != "" {
		return logs
	}
	return fallback
}

func pickRealtimeLogEntries(logger *infoLogger) []transport.LogEntry {
	if logger == nil {
		return nil
	}
	return logger.Entries()
}