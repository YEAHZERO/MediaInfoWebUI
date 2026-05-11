package screenshot

import (
	"os"
	"path/filepath"
	"strings"

	screenshotdvdinfo "mediainfo/internal/screenshot/dvdinfo"
	screenshotruntime "mediainfo/internal/screenshot/runtime"
	screenshotsource "mediainfo/internal/screenshot/source"
	screenshotsubtitle "mediainfo/internal/screenshot/subtitle"
	"mediainfo/internal/system"
)

func (r *screenshotRunner) init(timestamps []float64) error {
	if err := r.resolveRuntimeTools(); err != nil {
		return err
	}
	r.requested = timestamps
	if err := r.prepareOutputDir(); err != nil {
		return err
	}
	if err := r.prepareMediaTimeline(); err != nil {
		return err
	}
	if err := r.prepareSubtitlePipeline(); err != nil {
		return err
	}
	r.prepareRenderPipeline()
	return nil
}

func (r *screenshotRunner) resolveRuntimeTools() error {
	var err error

	r.tools.FFmpegBin, err = system.ResolveBin("FFMPEG_BIN", "ffmpeg")
	if err != nil {
		return err
	}
	r.tools.FFprobeBin, err = system.ResolveBin("FFPROBE_BIN", "ffprobe")
	if err != nil {
		return err
	}
	if bin, binErr := system.ResolveBin("MEDIAINFO_BIN", "mediainfo"); binErr == nil {
		r.tools.MediaInfoBin = bin
	}
	if bin, binErr := system.ResolveBin("OXIPNG_BIN", "oxipng"); binErr == nil {
		r.tools.OxiPNGBin = bin
	}
	if bin, binErr := system.ResolveBin("PNGQUANT_BIN", "pngquant"); binErr == nil {
		r.tools.PNGQuantBin = bin
	}
	return nil
}

func (r *screenshotRunner) prepareOutputDir() error {
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return err
	}
	return clearDir(r.outputDir)
}

func (r *screenshotRunner) prepareMediaTimeline() error {
	r.media.StartOffset = 0
	r.media.Duration = 0

	if screenshotsource.LooksLikeDVDSource(r.sourcePath) {
		r.ensureDVDMediaInfoResult()
	}
	return nil
}

func (r *screenshotRunner) prepareSubtitlePipeline() error {
	subtitleRunner := r.subtitleFlow()
	if r.subtitleMode != SubtitleModeOff {
		subtitleRunner.PrepareBlurayProbeContext()
	}
	if err := subtitleRunner.Choose(); err != nil {
		return err
	}
	if r.subtitle.Mode != "none" {
		if err := subtitleRunner.PrepareTextSubtitleRenderSource(); err != nil {
			return err
		}
		subtitleRunner.PrepareEmbeddedFonts()
		r.ensureSubtitleIndex()
	}
	return nil
}

func (r *screenshotRunner) prepareRenderPipeline() {
	if r.media.VideoWidth == 0 || r.media.VideoHeight == 0 {
		r.media.VideoWidth = 1920
		r.media.VideoHeight = 1080
	}
	r.render.ColorInfo = ""
	r.render.ColorChain = ""
	r.logf("[信息] 准备完成：%dx%d", r.media.VideoWidth, r.media.VideoHeight)
}

func (r *screenshotRunner) subtitleFlow() *screenshotsubtitle.Runner {
	return screenshotsubtitle.NewRunner(screenshotsubtitle.RunnerConfig{
		Ctx:                      r.ctx,
		SourcePath:               r.sourcePath,
		DVDMediaInfoPath:         r.dvdMediaInfoPath,
		SubtitleMode:             r.subtitleMode,
		Settings:                 r.settings,
		Tools:                    r.tools,
		Media:                    &r.media,
		SubtitleState:            &r.subtitleState,
		Subtitle:                 &r.subtitle,
		Logf:                     r.logf,
		LogProgress:              r.logProgress,
		LogProgressPercent:       r.logProgressPercent,
		StartHeartbeat:           r.startProgressHeartbeat,
		EnsureDVDMediaInfo:       r.ensureDVDMediaInfoResult,
		IsSupportedBitmap:        r.isSupportedBitmapSubtitle,
		RunFFmpegSubtitleExtract: r.runFFmpegSubtitleExtract,
	})
}

func (r *screenshotRunner) ensureSubtitleIndex() []screenshotruntime.SubtitleSpan {
	return r.subtitleFlow().EnsureIndex()
}

func (r *screenshotRunner) ensureDVDMediaInfoResult() (screenshotruntime.DVDMediaInfoResult, bool, error) {
	if r == nil || strings.TrimSpace(r.tools.MediaInfoBin) == "" {
		return screenshotruntime.DVDMediaInfoResult{}, false, nil
	}
	if !screenshotsource.LooksLikeDVDSource(r.sourcePath) {
		return screenshotruntime.DVDMediaInfoResult{}, false, nil
	}
	if r.subtitleState.HasDVDMediaInfoResult {
		return *r.subtitleState.DVDResult, true, nil
	}

	result, err := screenshotdvdinfo.Probe(r.ctx, r.tools.MediaInfoBin, r.sourcePath, r.dvdMediaInfoPath)
	if err != nil {
		return screenshotruntime.DVDMediaInfoResult{}, false, err
	}
	r.subtitleState.DVDResult = &result
	r.subtitleState.HasDVDMediaInfoResult = true
	return result, true, nil
}

func clearDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}