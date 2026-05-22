package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"mediainfo/internal/filelogger"
	"mediainfo/internal/httpapi/transport"
)

func DownloadLogsHandler(w http.ResponseWriter, r *http.Request) {
	logsDir := filelogger.LogsDir()

	var allLogs []byte
	now := time.Now()
	twentyFourHoursAgo := now.Add(-24 * time.Hour)

	err := filepath.WalkDir(logsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if info.ModTime().Before(twentyFourHoursAgo) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		allLogs = append(allLogs, []byte("===== ")...)
		allLogs = append(allLogs, []byte(d.Name())...)
		allLogs = append(allLogs, []byte(" =====\n")...)
		allLogs = append(allLogs, content...)
		allLogs = append(allLogs, []byte("\n\n")...)

		return nil
	})

	if err != nil {
		transport.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(allLogs) == 0 {
		allLogs = []byte("没有找到最近 24 小时的日志")
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	filename := fmt.Sprintf("mediainfo-logs-%s.txt", time.Now().Format("20060102"))
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Length", string(rune(len(allLogs))))
	w.WriteHeader(http.StatusOK)
	w.Write(allLogs)
}