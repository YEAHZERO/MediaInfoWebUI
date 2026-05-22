package hosting

import (
	"context"
	"strings"

	"mediainfo/internal/screenshot/pixhost"
)

type Pixhost struct{}

func NewPixhost() *Pixhost {
	return &Pixhost{}
}

func (p *Pixhost) Name() string {
	return "pixhost"
}

func (p *Pixhost) DisplayName() string {
	return "Pixhost"
}

func (p *Pixhost) Upload(ctx context.Context, imagePaths []string, onLog func(string)) ([]string, error) {
	var logs strings.Builder
	logHandler := func(msg string) {
		if onLog != nil {
			onLog(msg)
		}
		logs.WriteString(msg)
		logs.WriteString("\n")
	}

	result, err := pixhost.UploadImages(ctx, imagePaths, nil, 10*1024*1024, logHandler, nil)
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