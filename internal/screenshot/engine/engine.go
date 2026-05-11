package engine

import (
	"context"
	"strings"
)

const (
	VariantPNG = "png"
	VariantJPG = "jpg"

	SubtitleModeAuto = "auto"
	SubtitleModeOff  = "off"

	defaultScreenshotCount = 4
	minScreenshotCount     = 1
	maxScreenshotCount     = 10

	ProbeSizeDefault  = "150M"
	AnalyzeDefault    = "150M"
	CoarseBackDefault = 12
	FineSeekDefault   = 2
	coarseBackPGS     = 14
)

type ColorSpace int

const (
	ColorSpaceUnknown ColorSpace = iota
	ColorSpaceSDR
	ColorSpaceHDR10
	ColorSpaceHDR10Plus
	ColorSpaceDolbyVision
)

type ColorSpaceInfo struct {
	ColorSpace   string `json:"colorSpace,omitempty"`
	Primaries    string `json:"primaries,omitempty"`
	Transfer     string `json:"transfer,omitempty"`
	DolbyVision  bool   `json:"dolbyVision,omitempty"`
	DVProfile    int    `json:"dvProfile,omitempty"`
	ChainSummary string `json:"chainSummary,omitempty"`
}

type ProgressPhase string

const (
	PhaseProbe      ProgressPhase = "probe"
	PhaseTimestamps ProgressPhase = "timestamps"
	PhaseCapture    ProgressPhase = "capture"
	PhaseSubtitle   ProgressPhase = "subtitle"
	PhaseComposite  ProgressPhase = "composite"
	PhaseCompress   ProgressPhase = "compress"
	PhaseReencode   ProgressPhase = "reencode"
	PhaseDone       ProgressPhase = "done"
)

type ProgressEvent struct {
	Phase   ProgressPhase `json:"phase"`
	Current int           `json:"current"`
	Total   int           `json:"total"`
	Message string        `json:"message"`
}

type ProgressCallback func(event ProgressEvent)

type CaptureOptions struct {
	SourcePath   string
	OutputDir    string
	Variant      string
	SubtitleMode string
	Count        int
	OnProgress   ProgressCallback
}

type ScreenshotFileInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Name string `json:"name"`
}

type CaptureResult struct {
	Files       []ScreenshotFileInfo
	Logs        string
	ColorSpace  *ColorSpaceInfo
	Compression *CompressionResult
}

type CompressionResult struct {
	OriginalSize   int64
	Compressed     bool
	CompressedSize int64
	Method         string
	Lossy          bool
}

type CompressionOptions struct {
	Threshold int64
	Strategy  string
}

type SubtitleTrack struct {
	Index     int
	Codec     string
	Language  string
	Forced    bool
	IsDefault bool
}

type ScreenshotEngine interface {
	Capture(ctx context.Context, opts CaptureOptions) (*CaptureResult, error)
	DetectColorSpace(ctx context.Context, sourcePath string) (*ColorSpaceInfo, error)
	CompressIfNeeded(ctx context.Context, path string, threshold int64, strategy string) (*CompressionResult, error)
}

func NormalizeCount(raw string) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return defaultScreenshotCount
	}
	var count int
	if err := scanInt(value, &count); err != nil {
		return defaultScreenshotCount
	}
	if count < minScreenshotCount {
		return minScreenshotCount
	}
	if count > maxScreenshotCount {
		return maxScreenshotCount
	}
	return count
}

func NormalizeVariant(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case VariantJPG:
		return VariantJPG
	default:
		return VariantPNG
	}
}

func NormalizeSubtitleMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case SubtitleModeOff, "none", "nosub", "false", "0":
		return SubtitleModeOff
	default:
		return SubtitleModeAuto
	}
}

func scanInt(raw string, v *int) error {
	n := 0
	neg := false
	i := 0
	if i < len(raw) && raw[i] == '-' {
		neg = true
		i++
	}
	for ; i < len(raw); i++ {
		ch := raw[i]
		if ch < '0' || ch > '9' {
			return &strconvError{raw}
		}
		n = n*10 + int(ch-'0')
	}
	if neg {
		n = -n
	}
	*v = n
	return nil
}

type strconvError struct {
	raw string
}

func (e *strconvError) Error() string {
	return "invalid integer: " + e.raw
}