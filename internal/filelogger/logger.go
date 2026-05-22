package filelogger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Feature string

const (
	MediaInfo   Feature = "mediainfo"
	BDInfo      Feature = "bdinfo"
	Screenshots Feature = "screenshots"
)

var (
	logsDir string
	mu      sync.Mutex
	writers = make(map[Feature]*logWriter)
)

type logWriter struct {
	file     *os.File
	date     string
	feature  Feature
	basePath string
}

func SetLogsDir(dir string) {
	mu.Lock()
	defer mu.Unlock()
	logsDir = dir
	_ = os.MkdirAll(dir, 0755)
}

func Log(feature Feature, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, feature, msg)

	lw, err := getWriterLocked(feature)
	if err != nil {
		_ = os.WriteFile(filepath.Join(os.TempDir(), "mediainfo-log-fallback.txt"), []byte(line), 0644)
		return
	}
	_, _ = lw.file.WriteString(line)
}

func CloseAll() {
	mu.Lock()
	defer mu.Unlock()
	for _, w := range writers {
		if w.file != nil {
			_ = w.file.Close()
		}
	}
	writers = make(map[Feature]*logWriter)
}

func getWriterLocked(feature Feature) (*logWriter, error) {
	dir := logsDir
	if dir == "" {
		dir = filepath.Join(".", "logs")
	}
	_ = os.MkdirAll(dir, 0755)

	today := time.Now().Format("2006-01-02")

	if existing, ok := writers[feature]; ok {
		if existing.date == today {
			return existing, nil
		}
		_ = existing.file.Close()
		delete(writers, feature)
	}

	filename := fmt.Sprintf("%s-%s.log", feature, today)
	path := filepath.Join(dir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	lw := &logWriter{
		file:     f,
		date:     today,
		feature:  feature,
		basePath: path,
	}
	writers[feature] = lw
	return lw, nil
}

func LogsDir() string {
	mu.Lock()
	defer mu.Unlock()
	if logsDir == "" {
		return filepath.Join(".", "logs")
	}
	return logsDir
}