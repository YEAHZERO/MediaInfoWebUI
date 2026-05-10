package render

import (
	"context"
	"fmt"
	"strings"

	"mediainfo/internal/system"
)

type ColorSpaceInfo struct {
	Transfer   string
	ColorSpace string
	BitDepth   int
	DolbyVision bool
	HDR10       bool
}

func ProbeColorSpace(ctx context.Context, ffprobe, sourcePath string) (*ColorSpaceInfo, error) {
	stdout, stderr, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=color_transfer:stream=color_space:stream=bits_per_raw_sample",
		"-of", "csv=p=0",
		sourcePath,
	)
	if err != nil {
		return nil, system.NewCommandError(err, stdout, stderr)
	}

	info := &ColorSpaceInfo{}
	lines := strings.Split(stdout, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) >= 1 && info.Transfer == "" {
			info.Transfer = fields[0]
		}
		if len(fields) >= 2 && info.ColorSpace == "" {
			info.ColorSpace = fields[1]
		}
		if len(fields) >= 3 && info.BitDepth == 0 {
			var depth int
			fmt.Sscanf(fields[2], "%d", &depth)
			info.BitDepth = depth
		}
	}

	info.DolbyVision = strings.Contains(strings.ToLower(info.Transfer), "dolby")
	info.HDR10 = strings.Contains(strings.ToLower(info.Transfer), "smpte2084") || 
		strings.Contains(strings.ToLower(info.Transfer), "arib")

	return info, nil
}

func NeedsToneMapping(info *ColorSpaceInfo) bool {
	if info == nil {
		return false
	}
	return info.DolbyVision || info.HDR10 || info.BitDepth > 8
}