package timestamps

import (
	"context"
	"strconv"
	"strings"

	"mediainfo/internal/system"
)

func ParseDuration(stdout string) (float64, error) {
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strconv.ParseFloat(line, 64)
	}
	return 0, nil
}

func ProbeDuration(ctx context.Context, ffprobe, sourcePath string) (float64, error) {
	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		sourcePath,
	)
	if err != nil {
		return 0, err
	}
	return ParseDuration(stdout)
}