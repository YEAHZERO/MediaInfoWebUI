package pixhost

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type uploadBatch struct {
	onLog        LogHandler
	onItem       UploadItemHandler
	logLines     []string
	links        []string
	items        []UploadedImage
	lossySet     map[string]struct{}
	lossyIndexes []int
}

func newUploadBatch(lossyFiles []string, onLog LogHandler, onItem UploadItemHandler) *uploadBatch {
	lossySet := make(map[string]struct{}, len(lossyFiles))
	for _, name := range lossyFiles {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		lossySet[name] = struct{}{}
	}

	return &uploadBatch{
		onLog:        onLog,
		onItem:       onItem,
		logLines:     make([]string, 0),
		links:        make([]string, 0),
		items:        make([]UploadedImage, 0),
		lossySet:     lossySet,
		lossyIndexes: make([]int, 0, len(lossySet)),
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
	if _, ok := b.lossySet[filepath.Base(imagePath)]; ok {
		b.lossyIndexes = append(b.lossyIndexes, len(b.links))
	}

	b.links = append(b.links, directURL)
	item := buildUploadedImage(imagePath, directURL)
	b.items = append(b.items, item)
	if b.onItem != nil {
		b.onItem(item)
	}
	b.appendLog("已上传并校准域名: %s", filepath.Base(imagePath))
}

func (b *uploadBatch) finalize(total int) (Result, error) {
	if b == nil {
		return Result{}, errors.New("pixhost upload batch is nil")
	}

	b.appendLog("")
	b.appendLog("处理完成! 成功: %d/%d", len(b.links), total)
	if len(b.links) == 0 {
		return Result{Logs: b.logs(), Items: b.items, LossyIndexes: b.lossyIndexes}, errors.New("pixhost upload completed but returned no links")
	}

	output := strings.Join(extractDirectLinks(strings.Join(b.links, "\n")), "\n")
	return Result{
		Output:       output,
		Logs:         b.logs(),
		Items:        b.items,
		LossyIndexes: b.lossyIndexes,
	}, nil
}

func buildUploadedImage(imagePath, directURL string) UploadedImage {
	item := UploadedImage{
		URL:      strings.TrimSpace(directURL),
		Filename: filepath.Base(imagePath),
	}

	info, err := os.Stat(imagePath)
	if err == nil && !info.IsDir() && info.Size() > 0 {
		item.Size = info.Size()
	}
	return item
}

func collectUploadableImages(paths []string, maxUploadBytes int64) []string {
	candidates := make([]string, 0, len(paths))
	for _, path := range paths {
		if !isUploadableImage(path, maxUploadBytes) {
			continue
		}
		candidates = append(candidates, path)
	}
	sort.Strings(candidates)
	return candidates
}

func isUploadableImage(path string, maxUploadBytes int64) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if info.Size() <= 0 || (maxUploadBytes > 0 && info.Size() > maxUploadBytes) {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	header := make([]byte, 512)
	n, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false
	}

	contentType := http.DetectContentType(header[:n])
	return strings.HasPrefix(contentType, "image/")
}