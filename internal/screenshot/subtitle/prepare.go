package subtitle

import (
	"fmt"
	"os"
	"strings"

	"mediainfo/internal/system"
)

func (r *Runner) PrepareTextSubtitleRenderSource() error {
	selection := r.selection()
	if selection.Mode != "internal" {
		return nil
	}
	if r.isSupportedBitmapSubtitle() {
		return nil
	}
	if !IsSupportedTextCodec(selection.Codec) {
		return fmt.Errorf("unsupported text subtitle codec: %s", FormatLabel(selection.Codec))
	}
	if !r.ShouldExtractInternalTextSubtitle() {
		return nil
	}

	pattern, extractionArgs, _, logMessage := InternalTextSubtitleExtractionPlan(selection.Codec)
	r.logProgress("字幕", 3, 3, "正在提取内封文字字幕。")
	r.logf("%s", logMessage)

	tempFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	if closeErr := tempFile.Close(); closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}

	stdout, stderr, err := r.runSubtitleExtract([]string{
		"-v", "error",
		"-i", r.SourcePath,
		"-map", fmt.Sprintf("0:s:%d", selection.RelativeIndex),
		"-c:s", extractionArgs,
		"-y", tempPath,
	})
	if err != nil {
		_ = os.Remove(tempPath)
		if message := strings.TrimSpace(system.BestErrorMessage(err, stderr, stdout)); message != "" {
			normalized := strings.ReplaceAll(message, "\r\n", "\n")
			normalized = strings.ReplaceAll(normalized, "\r", "\n")
			for _, line := range strings.Split(normalized, "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				r.logf("[错误] 提取失败详情: %s", line)
			}
		}
		return fmt.Errorf("failed to extract internal text subtitle: %w", err)
	}

	state := r.state()
	state.TempSubtitleFile = tempPath
	selection.ExtractedText = true
	r.logf("[信息] 已提取内封文本字幕供截图使用：%s", tempPath)
	return nil
}

func (r *Runner) ShouldExtractInternalTextSubtitle() bool {
	selection := r.selection()
	if selection.Mode != "internal" {
		return false
	}
	if r.isSupportedBitmapSubtitle() {
		return false
	}
	return true
}

func InternalTextSubtitleExtractionPlan(codec string) (pattern string, extractionCodecArg string, extractedCodec string, logMessage string) {
	switch strings.ToLower(strings.TrimSpace(codec)) {
	case "ass":
		return "minfo-sub-*.ass", "copy", "ass", "[信息] 内封 ASS 字幕将提取为原始 ASS 文件，保留样式后参与截图渲染。"
	case "ssa":
		return "minfo-sub-*.ssa", "copy", "ssa", "[信息] 内封 SSA 字幕将提取为原始 SSA 文件，保留样式后参与截图渲染。"
	default:
		return "minfo-sub-*.srt", "srt", "subrip", "[信息] 内封文字字幕将先提取为临时字幕文件，再参与截图渲染。"
	}
}