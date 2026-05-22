package hosting

import (
	"context"
	"strings"

	"mediainfo/internal/screenshot/freeimage"
)

type Freeimage struct{}

func NewFreeimage() *Freeimage {
	return &Freeimage{}
}

func (f *Freeimage) Name() string {
	return "freeimage"
}

func (f *Freeimage) DisplayName() string {
	return "Freeimage.host"
}

func (f *Freeimage) Upload(ctx context.Context, imagePaths []string, onLog func(string)) ([]string, error) {
	var logs strings.Builder
	logHandler := func(msg string) {
		if onLog != nil {
			onLog(msg)
		}
		logs.WriteString(msg)
		logs.WriteString("\n")
	}

	result, err := freeimage.UploadImages(ctx, imagePaths, logHandler, nil)
	if err != nil {
		return nil, err
	}

	if result.Output == "" {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	links := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			links = append(links, line)
		}
	}
	return links, nil
}