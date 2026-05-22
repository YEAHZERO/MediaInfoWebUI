package freeimage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type UploadedImage struct {
	URL      string
	Filename string
	Size     int64
}

type UploadItemHandler func(item UploadedImage)

type LogHandler func(line string)

type Result struct {
	Output string
	Logs   string
	Items  []UploadedImage
}

func UploadImages(ctx context.Context, files []string, onLog LogHandler, onItem UploadItemHandler) (Result, error) {
	images := collectUploadableImages(files)
	batch := newUploadBatch(onLog, onItem)
	if len(images) == 0 {
		batch.appendLog("警告: 未找到有效图片文件")
		return Result{Logs: batch.logs()}, errors.New("no uploadable screenshots were found")
	}

	batch.appendLog("开始上传 %d 个文件到 Freeimage.host...", len(images))
	client := &http.Client{}
	for _, imagePath := range images {
		directURL, err := UploadImage(ctx, client, imagePath)
		if err != nil {
			batch.recordFailure(imagePath, err)
			continue
		}
		batch.recordSuccess(imagePath, directURL)
	}

	return batch.finalize(len(images))
}

type uploadBatch struct {
	onLog    LogHandler
	onItem   UploadItemHandler
	logLines []string
	links    []string
	items    []UploadedImage
}

func newUploadBatch(onLog LogHandler, onItem UploadItemHandler) *uploadBatch {
	return &uploadBatch{
		onLog:    onLog,
		onItem:   onItem,
		logLines: make([]string, 0),
		links:    make([]string, 0),
		items:    make([]UploadedImage, 0),
	}
}

func (b *uploadBatch) logs() string {
	if b == nil {
		return ""
	}
	return strings.Join(b.logLines, "\n")
}

func (b *uploadBatch) appendLog(format string, args ...any) {
	if b == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	b.logLines = append(b.logLines, line)
	if b.onLog != nil {
		b.onLog(line)
	}
}

func (b *uploadBatch) recordFailure(imagePath string, err error) {
	if b == nil {
		return
	}
	b.appendLog("上传失败: %s (%s)", filepath.Base(imagePath), err.Error())
}

func (b *uploadBatch) recordSuccess(imagePath, directURL string) {
	if b == nil {
		return
	}
	b.links = append(b.links, directURL)
	item := UploadedImage{
		URL:      directURL,
		Filename: filepath.Base(imagePath),
	}
	if info, err := os.Stat(imagePath); err == nil && !info.IsDir() && info.Size() > 0 {
		item.Size = info.Size()
	}
	b.items = append(b.items, item)
	if b.onItem != nil {
		b.onItem(item)
	}
	b.appendLog("已上传: %s", filepath.Base(imagePath))
}

func (b *uploadBatch) finalize(total int) (Result, error) {
	if b == nil {
		return Result{}, errors.New("freeimage upload batch is nil")
	}

	b.appendLog("")
	b.appendLog("处理完成! 成功: %d/%d", len(b.links), total)
	if len(b.links) == 0 {
		return Result{Logs: b.logs(), Items: b.items}, errors.New("freeimage.host upload completed but returned no links")
	}

	output := strings.Join(b.links, "\n")
	return Result{
		Output: output,
		Logs:   b.logs(),
		Items:  b.items,
	}, nil
}

func collectUploadableImages(paths []string) []string {
	candidates := make([]string, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() || info.Size() <= 0 {
			continue
		}
		candidates = append(candidates, path)
	}
	return candidates
}