package runtime

import "strings"

// LineHandler 处理截图流程产生的单行实时日志。
type LineHandler func(line string)

type SubtitleSpan struct {
	Start float64
	End   float64
}

type FFprobePacket struct {
	PTSTime      string `json:"pts_time"`
	DurationTime string `json:"duration_time"`
	Size         string `json:"size"`
}

type DVDMediaInfoResult struct {
	Duration              float64
	DisplayAspectRatio    string
	Tracks                []DVDMediaInfoTrack
	ProbePath             string
	SelectedVOBPath       string
	LanguageFallbackPath  string
}

type DVDMediaInfoTrack struct {
	StreamID int
	ID       string
	Format   string
	Language string
	Title    string
	Source   string
}

type VariantSettings struct {
	Ext            string
	ProbeSize      string
	Analyze        string
	CoarseBackText int
	CoarseBackPGS  int
	RenderBackText int
	RenderBackPGS  int
	SearchBack     float64
	SearchForward  float64
	JPGQuality     int
}

func VariantSettingsFor(variant string) VariantSettings {
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "jpg":
		return VariantSettings{
			Ext:            ".jpg",
			ProbeSize:      "100M",
			Analyze:        "100M",
			CoarseBackText: 2,
			CoarseBackPGS:  8,
			RenderBackText: 1,
			RenderBackPGS:  2,
			SearchBack:     4,
			SearchForward:  8,
			JPGQuality:     1,
		}
	default:
		return VariantSettings{
			Ext:            ".png",
			ProbeSize:      "150M",
			Analyze:        "150M",
			CoarseBackText: 3,
			CoarseBackPGS:  12,
			RenderBackText: 1,
			RenderBackPGS:  2,
			SearchBack:     6,
			SearchForward:  10,
			JPGQuality:     85,
		}
	}
}

type BitmapSubtitleKind string

const (
	BitmapSubtitleNone BitmapSubtitleKind = ""
	BitmapSubtitlePGS  BitmapSubtitleKind = "pgs"
	BitmapSubtitleDVD  BitmapSubtitleKind = "dvd"
)

type SubtitleSelection struct {
	Mode          string
	File          string
	StreamIndex   int
	RelativeIndex int
	Lang          string
	Codec         string
	Title         string
	ExtractedText bool
}

type SubtitleTrack struct {
	Index     int
	StreamID  string
	Codec     string
	Language  string
	Title     string
	Forced    int
	IsDefault int
	Tags      string
}

type SubtitleState struct {
	BlurayContext            BlurayProbeContext
	HasDVDMediaInfoResult    bool
	TempSubtitleFile         string
	DVDResult                *DVDMediaInfoResult
	Selection                *SubtitleSelection
	SubtitleFontDir          string
	Index                    []SubtitleSpan
	IndexBuilt               bool
	RejectedBitmapCandidates map[string]struct{}
	BitmapRenderBackOverride int
}

type BlurayProbeContext struct {
	Root     string
	Playlist string
	Clip     string
}

type BlurayHelperTrack struct {
	PID          int    `json:"pid"`
	Lang         string `json:"lang"`
	CodingType   int    `json:"coding_type"`
	CharCode     int    `json:"char_code"`
	SubpathID    int    `json:"subpath_id"`
	PayloadBytes uint64 `json:"payload_bytes"`
	Bitrate      int64  `json:"bitrate"`
}

type BlurayHelperClip struct {
	ClipID        string              `json:"clip_id"`
	PGStreamCount int                 `json:"pg_stream_count"`
	PacketSeconds float64             `json:"packet_seconds"`
	PGStreams     []BlurayHelperTrack `json:"pg_streams"`
}

type BlurayHelperResult struct {
	Source         string           `json:"source"`
	BitrateScanned bool             `json:"bitrate_scanned"`
	BitrateMode    string           `json:"bitrate_mode"`
	Clip           BlurayHelperClip `json:"clip"`
}

type PreferredSubtitleRank struct {
	LangClass        string
	LangScore        int
	DispositionScore int
	PID              int
	PIDOK            bool
	BitmapKind       BitmapSubtitleKind
	PayloadBytes     uint64
	UsePayloadBytes  bool
	Bitrate          int64
	UseBitrate       bool
}

type StreamDisposition struct {
	Default int `json:"default"`
	Forced  int `json:"forced"`
}

type FFprobeSubtitleStream struct {
	Index       int                    `json:"index"`
	ID          interface{}            `json:"id"`
	CodecName   string                 `json:"codec_name"`
	Tags        map[string]interface{} `json:"tags"`
	Disposition StreamDisposition      `json:"disposition"`
}

type FFprobeStreamsPayload struct {
	Streams []FFprobeSubtitleStream `json:"streams"`
}

type FFprobePacketsPayload struct {
	Packets []FFprobePacket `json:"packets"`
}

type Toolchain struct {
	FFprobeBin      string
	FFmpegBin       string
	MediaInfoBin    string
	BDSubBin        string
	PixHostKey      string
	OxiPNGBin       string
	PNGQuantBin     string
	LibplaceboReady bool
}

type MediaState struct {
	Duration      float64
	StartOffset   float64
	VideoWidth    int
	VideoHeight   int
	DisplayWidth  int
	DisplayHeight int
}

type RenderState struct {
	SubtitleCanvasWidth  int
	SubtitleCanvasHeight int
	AspectChain          string
	ColorInfo            string
	ColorChain           string
}

type ScreenshotsResult struct {
	Files           []string
	Logs            string
	Items           []UploadedImage
	LossyPNGFiles   []string
	LossyPNGIndexes []int
	Output          string
	DownloadURL     string
	Size            int64
}

type UploadedImage struct {
	URL      string `json:"url"`
	Delete   string `json:"delete"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
}