package pixhost

import (
	"context"
	"errors"
	"net/http"
)

func UploadImages(ctx context.Context, files, lossyFiles []string, maxUploadBytes int64, onLog LogHandler, onItem UploadItemHandler) (Result, error) {
	images := collectUploadableImages(files, maxUploadBytes)
	batch := newUploadBatch(lossyFiles, onLog, onItem)
	if len(images) == 0 {
		batch.appendLog("警告: 未找到有效图片文件")
		return Result{Logs: batch.logs()}, errors.New("no uploadable screenshots were found")
	}

	batch.appendLog("开始处理 %d 个文件...", len(images))
	client := &http.Client{}
	apiURL := endpoint()
	for _, imagePath := range images {
		directURL, err := uploadSingleImage(ctx, client, apiURL, imagePath)
		if err != nil {
			batch.recordFailure(imagePath, err)
			continue
		}
		batch.recordSuccess(imagePath, directURL)
	}

	return batch.finalize(len(images))
}

func extractDirectLinks(output string) []string {
	links := make([]string, 0)
	for _, line := range splitLines(output) {
		if hasHTTPPrefix(line) {
			links = append(links, line)
		}
	}
	return links
}

func splitLines(text string) []string {
	lines := make([]string, 0)
	start := 0
	for index := 0; index <= len(text); index++ {
		if index < len(text) && text[index] != '\n' {
			continue
		}
		line := text[start:index]
		for len(line) > 0 && (line[len(line)-1] == '\r' || line[len(line)-1] == '\n') {
			line = line[:len(line)-1]
		}
		for len(line) > 0 && (line[0] == '\r' || line[0] == '\n') {
			line = line[1:]
		}
		if line != "" {
			lines = append(lines, line)
		}
		start = index + 1
	}
	return lines
}

func hasHTTPPrefix(value string) bool {
	return len(value) >= len("http://") && (value[:7] == "http://" || (len(value) >= len("https://") && value[:8] == "https://"))
}