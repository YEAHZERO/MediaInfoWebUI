package screenshot

import (
	"errors"
	"fmt"
	"path/filepath"

	screenshotdelivery "mediainfo/internal/screenshot/delivery"
	screenshottimestamps "mediainfo/internal/screenshot/timestamps"
)

type screenshotRunState struct {
	totalShots     int
	startedShots   int
	processedShots int
	successCount   int
	failures       []string
	usedNames      map[string]int
	usedSeconds    map[int]struct{}
}

type screenshotCapturePlan struct {
	aligned    float64
	outputName string
	outputPath string
}

func newScreenshotRunState(totalShots int) *screenshotRunState {
	return &screenshotRunState{
		totalShots:  totalShots,
		failures:    make([]string, 0),
		usedNames:   make(map[string]int, totalShots),
		usedSeconds: make(map[int]struct{}, totalShots),
	}
}

func (s *screenshotRunState) nextShotIndex() int {
	return s.startedShots + 1
}

func (s *screenshotRunState) beginShot() int {
	s.startedShots++
	return s.startedShots
}

func (s *screenshotRunState) markFailed(outputName string, err error) int {
	s.failures = append(s.failures, fmt.Sprintf("[失败] 文件: %s\n原因: %s", outputName, err.Error()))
	s.processedShots++
	return s.processedShots
}

func (s *screenshotRunState) markSucceeded(aligned float64) int {
	s.usedSeconds[int(aligned)] = struct{}{}
	s.successCount++
	s.processedShots++
	return s.processedShots
}

func (r *screenshotRunner) run() ([]string, error) {
	state := newScreenshotRunState(len(r.requested))
	for _, requested := range r.requested {
		r.runRequestedScreenshot(requested, state)
	}
	return r.finalizeScreenshotRun(state)
}

func (r *screenshotRunner) runRequestedScreenshot(requested float64, state *screenshotRunState) {
	plan, ok := r.prepareScreenshotCapture(requested, state)
	if !ok {
		return
	}
	r.capturePreparedScreenshot(plan, state)
}

func (r *screenshotRunner) prepareScreenshotCapture(requested float64, state *screenshotRunState) (screenshotCapturePlan, bool) {
	r.activeShot.Prepare(state.nextShotIndex(), state.totalShots)

	aligned, ok := r.resolveAlignedScreenshotTime(requested, state)
	if !ok {
		r.activeShot.Reset()
		return screenshotCapturePlan{}, false
	}

	ts := int(aligned)
	outputName := fmt.Sprintf("%dmin%02ds.png", ts/60, ts%60)
	if ts < 60 {
		outputName = fmt.Sprintf("%ds.png", ts)
	}
	outputPath := filepath.Join(r.outputDir, outputName)
	r.logf("[信息] 截图: 请求 %s → 对齐 %s → 输出 %s",
		screenshottimestamps.SecToHMSMS(requested),
		screenshottimestamps.SecToHMSMS(aligned),
		outputName,
	)

	current := state.beginShot()
	r.activeShot.BeginRender(current, state.totalShots, outputName)
	r.logProgress("截图开始", current, state.totalShots, fmt.Sprintf("正在渲染第 %d/%d 张截图：%s", current, state.totalShots, outputName))

	return screenshotCapturePlan{
		aligned:    aligned,
		outputName: outputName,
		outputPath: outputPath,
	}, true
}

func (r *screenshotRunner) resolveAlignedScreenshotTime(requested float64, state *screenshotRunState) (float64, bool) {
	aligned := requested
	if r.subtitle.Mode != "none" {
		aligned = r.alignToSubtitle(requested)
	}
	aligned = r.clampToDuration(aligned)

	candidate, adjusted, ok := r.resolveUniqueScreenshotSecond(requested, aligned, state.usedSeconds)
	if !ok {
		r.logf("[提示] 请求 %.3fs 对齐后未找到新的唯一秒，跳过该截图。", requested)
		return 0, false
	}
	if adjusted {
		r.logf("[提示] 请求 %.3fs 对齐后命中已使用秒，改用唯一秒 %.3fs", requested, candidate)
	}
	return candidate, true
}

func (r *screenshotRunner) capturePreparedScreenshot(plan screenshotCapturePlan, state *screenshotRunState) {
	defer r.activeShot.Reset()

	if err := r.captureScreenshot(plan.aligned, plan.outputPath); err != nil {
		processed := state.markFailed(plan.outputName, err)
		r.logProgress("截图完成", processed, state.totalShots, fmt.Sprintf("第 %d/%d 张截图失败：%s", processed, state.totalShots, plan.outputName))
		return
	}

	processed := state.markSucceeded(plan.aligned)
	r.logProgress("截图完成", processed, state.totalShots, fmt.Sprintf("已完成第 %d/%d 张截图：%s", processed, state.totalShots, plan.outputName))
}

func (r *screenshotRunner) finalizeScreenshotRun(state *screenshotRunState) ([]string, error) {
	r.logScreenshotRunSummary(state)

	r.logProgress("整理", 1, 4, "正在整理截图文件列表。")
	files, err := screenshotdelivery.ListImageFiles(r.outputDir)
	if err != nil {
		if state.successCount == 0 {
			return nil, errors.New("no screenshots were generated")
		}
		return nil, err
	}
	return files, nil
}