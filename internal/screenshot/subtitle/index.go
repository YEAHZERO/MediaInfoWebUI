package subtitle

import (
	"context"
	"fmt"
	"math"

	screenshotruntime "mediainfo/internal/screenshot/runtime"
)

type SubtitleState struct {
	Index                    []screenshotruntime.SubtitleSpan
	IndexBuilt               bool
	RejectedBitmapCandidates map[string]struct{}
	BitmapRenderBackOverride int
}

type SubtitleSelection struct {
	Mode           string
	File           string
	RelativeIndex  int
	ExtractedText  bool
	SelectedCodec  string
}

type Runner struct {
	Ctx            context.Context
	SourcePath     string
	Tools          struct{
		FFprobeBin  string
		FFmpegBin   string
	}
	Settings         struct{
		ProbeSize      string
		Analyze        string
		CoarseBackPGS  int
	}
	subtitle      SubtitleSelection
	media         struct{
		Duration      float64
		StartOffset   float64
	}
	subtitleState SubtitleState
	logf          func(string, ...any)
	logProgress   func(string, int, int, string)
	logProgressPercent func(string, float64, string)
	startHeartbeat func(string, string) func()
	renderCoarseBack func() int
	isSupportedBitmapSubtitle func() bool
	isPGSSubtitle func() bool
	isDVDSubtitle func() bool
	bitmapSubtitleVisibleAt func(float64) (bool, error)
	internalBitmapSubtitleVisibleAtWithCoarseBack func(float64, int) (bool, error)
	logBitmapSubtitleVisibilityProgress func()
	ensureSubtitleIndex func() []screenshotruntime.SubtitleSpan
	selection func() SubtitleSelection
	state func() *SubtitleState
	mediaInfo func() struct{Duration float64;StartOffset float64}
}

func BitmapCandidateKey(t float64) string {
	return fmt.Sprintf("%.3f", math.Round(t*1000)/1000)
}

func DetectSubtitleType(ctx context.Context, ffprobe, sourcePath string) string {
	best, err := SelectBestSubtitle(ctx, ffprobe, sourcePath)
	if err != nil || best == nil {
		return ""
	}
	return GetSubtitleType(best.CodecName)
}

func DetectSubtitleRelativeIndex(ctx context.Context, ffprobe, sourcePath string) int {
	best, err := SelectBestSubtitle(ctx, ffprobe, sourcePath)
	if err != nil || best == nil {
		return -1
	}
	return GetRelativeIndex(ctx, ffprobe, sourcePath, best.CodecName)
}