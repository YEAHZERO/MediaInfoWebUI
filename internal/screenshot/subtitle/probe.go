package subtitle

import (
	"context"
	"fmt"
	"strings"

	"mediainfo/internal/system"
)

type SubtitleInfo struct {
	Type      string
	Index     int
	CodecName string
	Language  string
	Forced    bool
	Default   bool
}

func ProbeSubtitles(ctx context.Context, ffprobe, sourcePath string) ([]SubtitleInfo, error) {
	stdout, stderr, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=codec_name:stream=index:stream=language:stream=disposition",
		"-of", "csv=p=0",
		sourcePath,
	)
	if err != nil {
		return nil, system.NewCommandError(err, stdout, stderr)
	}

	var subs []SubtitleInfo
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 4 {
			continue
		}
		subs = append(subs, SubtitleInfo{
			CodecName: fields[0],
			Index:     parseInt(fields[1]),
			Language:  fields[2],
			Forced:    parseDisposition(fields[3], "forced"),
			Default:   parseDisposition(fields[3], "default"),
		})
	}
	return subs, nil
}

func parseInt(s string) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return -1
	}
	return n
}

func parseDisposition(disp, flag string) bool {
	flags := strings.Split(disp, "|")
	for _, f := range flags {
		if strings.Contains(f, flag) {
			return true
		}
	}
	return false
}