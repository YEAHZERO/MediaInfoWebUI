package runtime

type SubtitleSpan struct {
	Start float64
	End   float64
}

type FFprobePacket struct {
	PTSTime      string
	DurationTime string
	Size         string
}

type DVDMediaInfoResult struct {
	Duration         float64
	DisplayAspectRatio string
	Tracks           []DVDMediaInfoTrack
	ProbePath        string
	SelectedVOBPath  string
	LanguageFallbackPath string
}

type DVDMediaInfoTrack struct {
	StreamID int
	ID       string
	Format   string
	Language string
	Title    string
	Source   string
}
