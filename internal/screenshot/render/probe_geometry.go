package render

import (
	"context"
	"strconv"
	"strings"

	"mediainfo/internal/system"
)

type VideoGeometry struct {
	Width     int
	Height    int
	DAR       float64
	SAR       float64
	Rotation  int
}

func ProbeGeometry(ctx context.Context, ffprobe, sourcePath string) (*VideoGeometry, error) {
	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width:stream=height:stream=display_aspect_ratio:stream=sample_aspect_ratio:stream=rotation",
		"-of", "csv=p=0",
		sourcePath,
	)
	if err != nil {
		return nil, err
	}

	geom := &VideoGeometry{}
	lines := strings.Split(stdout, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) >= 1 {
			geom.Width = parseInt(fields[0])
		}
		if len(fields) >= 2 {
			geom.Height = parseInt(fields[1])
		}
		if len(fields) >= 3 && fields[2] != "N/A" {
			geom.DAR = parseRatio(fields[2])
		}
		if len(fields) >= 4 && fields[3] != "N/A" {
			geom.SAR = parseRatio(fields[3])
		}
		if len(fields) >= 5 {
			geom.Rotation = parseInt(fields[4])
		}
	}

	return geom, nil
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func parseRatio(s string) float64 {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 1.0
	}
	num, _ := strconv.ParseFloat(parts[0], 64)
	den, _ := strconv.ParseFloat(parts[1], 64)
	if den == 0 {
		return 1.0
	}
	return num / den
}