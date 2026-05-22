package hosting

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
)

type Local struct{}

func NewLocal() *Local {
	return &Local{}
}

func (l *Local) Name() string {
	return "local"
}

func (l *Local) DisplayName() string {
	return "本地"
}

func (l *Local) Upload(ctx context.Context, imagePaths []string, onLog func(string)) ([]string, error) {
	if onLog != nil {
		onLog("使用本地图床模式")
	}

	links := make([]string, 0, len(imagePaths))
	for _, path := range imagePaths {
		encodedPath := url.PathEscape(filepath.Base(path))
		links = append(links, fmt.Sprintf("/screenshots/local/%s", encodedPath))
	}
	
	if onLog != nil {
		onLog(fmt.Sprintf("生成了 %d 个本地链接", len(links)))
	}

	return links, nil
}