package screenshot

import (
	"fmt"

	screenshottimestamps "mediainfo/internal/screenshot/timestamps"
	"mediainfo/internal/system"
)

func (r *screenshotRunner) bitmapSubtitleVisibleAt(aligned float64) (bool, error) {
	if !r.isSupportedBitmapSubtitle() || r.subtitle.Mode != "internal" {
		return false, nil
	}

	switch {
	case r.isPGSSubtitle():
		return r.pgsSubtitleVisibleAt(aligned)
	case r.isDVDSubtitle():
		return r.dvdSubtitleVisibleAt(aligned)
	default:
		return false, nil
	}
}

func (r *screenshotRunner) pgsSubtitleVisibleAt(aligned float64) (bool, error) {
	return r.internalBitmapSubtitleVisibleAt(aligned)
}

func (r *screenshotRunner) dvdSubtitleVisibleAt(aligned float64) (bool, error) {
	return r.internalBitmapSubtitleVisibleAt(aligned)
}

func (r *screenshotRunner) internalBitmapSubtitleVisibleAt(aligned float64) (bool, error) {
	return r.internalBitmapSubtitleVisibleAtWithCoarseBack(aligned, r.renderCoarseBack())
}

func (r *screenshotRunner) internalBitmapSubtitleVisibleAtWithCoarseBack(aligned float64, coarseBack int) (bool, error) {
	baseFrame, err := r.captureBitmapProbeFrame(r.sourcePath, aligned, false, coarseBack)
	if err != nil {
		return false, err
	}
	subFrame, err := r.captureBitmapProbeFrame(r.sourcePath, aligned, true, coarseBack)
	if err != nil {
		return false, err
	}
	return baseFrame != subFrame, nil
}

func (r *screenshotRunner) captureBitmapProbeFrame(inputPath string, localTime float64, withSubtitle bool, coarseBack int) (string, error) {
	if coarseBack <= 0 {
		coarseBack = r.settings.CoarseBackPGS
	}
	_, fineSecond, coarseHMS := r.splitCaptureTimeline(localTime, coarseBack)

	args := []string{
		"-v", "error",
		"-fflags", "+genpts",
		"-ss", coarseHMS,
		"-probesize", r.settings.ProbeSize,
		"-analyzeduration", r.settings.Analyze,
		"-i", inputPath,
		"-ss", screenshottimestamps.FormatSeconds(fineSecond),
		"-frames:v", "1",
		"-f", "rawvideo",
		"-pix_fmt", "gray",
	}

	if withSubtitle {
		args = append(args, r.bitmapProbeOutputArgs()...)
	} else {
		filterChain := joinFilters(r.displayAspectFilter(), "format=gray")
		args = append(args,
			"-map", "0:v:0",
			"-vf", filterChain,
			"-",
		)
	}

	stdout, stderr, err := system.RunCommand(r.ctx, r.tools.FFmpegBin, args...)
	if err != nil {
		return "", fmt.Errorf(system.BestErrorMessage(err, stderr, stdout))
	}
	return stdout, nil
}

func (r *screenshotRunner) bitmapProbeOutputArgs() []string {
	filterComplex := r.buildInternalBitmapProbeFilterComplex()
	if r.isPGSSubtitle() {
		filterComplex = r.buildPGSOverlayFilterComplex(r.displayAspectFilter(), "format=gray")
	}
	return []string{
		"-filter_complex", filterComplex,
		"-map", "[out]",
		"-",
	}
}

func (r *screenshotRunner) buildInternalBitmapProbeFilterComplex() string {
	return fmt.Sprintf("[0:v:0][0:s:%d]overlay=(W-w)/2:(H-h-10),%s,format=gray[out]",
		r.subtitle.RelativeIndex,
		r.displayAspectFilter(),
	)
}

func (r *screenshotRunner) renderCoarseBack() int {
	if r == nil {
		return 1
	}
	if r.subtitle.Mode == "internal" && r.isSupportedBitmapSubtitle() {
		if r.subtitleState.BitmapRenderBackOverride > 0 {
			return r.subtitleState.BitmapRenderBackOverride
		}
		if r.settings.RenderBackPGS > 0 {
			return r.settings.RenderBackPGS
		}
		return r.settings.CoarseBackPGS
	}
	if r.settings.RenderBackText > 0 {
		return r.settings.RenderBackText
	}
	return r.settings.CoarseBackText
}

func (r *screenshotRunner) splitCaptureTimeline(aligned float64, coarseBack int) (int, float64, string) {
	coarseSecond := int(aligned) - coarseBack
	if coarseSecond < 0 {
		coarseSecond = 0
	}
	fineSecond := aligned - float64(coarseSecond)
	coarseHMS := screenshottimestamps.SecToHMS(float64(coarseSecond))
	return coarseSecond, fineSecond, coarseHMS
}
