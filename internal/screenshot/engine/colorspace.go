package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"mediainfo/internal/system"
)

func DetectColorSpace(ctx context.Context, sourcePath string) (*ColorSpaceInfo, error) {
	ffprobe, err := system.ResolveBin("FFPROBE_BIN", "ffprobe")
	if err != nil {
		return nil, err
	}

	info := probeFFprobeJSON(ctx, ffprobe, sourcePath)
	if info == nil {
		info = probeFFprobePlain(ctx, ffprobe, sourcePath)
	}
	if info == nil {
		return &ColorSpaceInfo{}, nil
	}

	info.ChainSummary = buildChainSummary(info)
	return info, nil
}

func probeFFprobeJSON(ctx context.Context, ffprobe, sourcePath string) *ColorSpaceInfo {
	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=color_space,color_primaries,color_transfer:stream_side_data_list=side_data_type,dv_profile",
		"-of", "json",
		sourcePath,
	)
	if err != nil || strings.TrimSpace(stdout) == "" {
		return nil
	}

	type sideDataEntry struct {
		SideDataType string `json:"side_data_type"`
		DVProfile    int    `json:"dv_profile"`
	}
	type colorStream struct {
		ColorSpace     string           `json:"color_space"`
		ColorPrimaries string           `json:"color_primaries"`
		ColorTransfer  string           `json:"color_transfer"`
		SideDataList   []sideDataEntry  `json:"side_data_list"`
	}
	var payload struct {
		Streams []colorStream `json:"streams"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return nil
	}
	if len(payload.Streams) == 0 {
		return nil
	}

	stream := payload.Streams[0]
	info := &ColorSpaceInfo{
		ColorSpace:  strings.TrimSpace(stream.ColorSpace),
		Primaries:   strings.TrimSpace(stream.ColorPrimaries),
		Transfer:    strings.TrimSpace(stream.ColorTransfer),
	}

	for _, sd := range stream.SideDataList {
		lowerType := strings.ToLower(strings.TrimSpace(sd.SideDataType))
		if strings.Contains(lowerType, "dovi") || strings.Contains(lowerType, "dolby vision") {
			info.DolbyVision = true
			if sd.DVProfile > 0 {
				info.DVProfile = sd.DVProfile
			}
			break
		}
	}
	return info
}

func probeFFprobePlain(ctx context.Context, ffprobe, sourcePath string) *ColorSpaceInfo {
	stdout, _, err := system.RunCommand(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=color_space,color_primaries,color_transfer",
		"-of", "default=noprint_wrappers=1",
		sourcePath,
	)
	if err != nil {
		return nil
	}

	info := &ColorSpaceInfo{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "color_space="):
			info.ColorSpace = strings.TrimPrefix(line, "color_space=")
		case strings.HasPrefix(line, "color_primaries="):
			info.Primaries = strings.TrimPrefix(line, "color_primaries=")
		case strings.HasPrefix(line, "color_transfer="):
			info.Transfer = strings.TrimPrefix(line, "color_transfer=")
		}
	}

	if info.ColorSpace == "" && info.Primaries == "" && info.Transfer == "" {
		return nil
	}
	return info
}

func buildChainSummary(info *ColorSpaceInfo) string {
	if info == nil {
		return ""
	}

	if info.DolbyVision {
		profile := ""
		if info.DVProfile > 0 {
			profile = fmt.Sprintf(" Profile %d", info.DVProfile)
		}
		return fmt.Sprintf("Dolby Vision%s — requires tone mapping", profile)
	}

	hasHDR := strings.Contains(info.ColorSpace, "bt2020") ||
		strings.Contains(info.Primaries, "bt2020")
	hasPQ := strings.Contains(info.Transfer, "smpte2084")
	hasHLG := strings.Contains(info.Transfer, "arib-std-b67") ||
		strings.Contains(info.Transfer, "arib")

	if hasHDR && hasPQ {
		return "HDR10 — requires tone mapping"
	}
	if hasHDR && hasHLG {
		return "HLG/HDR — requires tone mapping"
	}
	if hasHDR {
		return "Wide color gamut — may benefit from tone mapping"
	}

	lines := make([]string, 0, 3)
	if info.ColorSpace != "" {
		lines = append(lines, "color_space="+info.ColorSpace)
	}
	if info.Primaries != "" {
		lines = append(lines, "color_primaries="+info.Primaries)
	}
	if info.Transfer != "" {
		lines = append(lines, "color_transfer="+info.Transfer)
	}
	sort.Strings(lines)
	if len(lines) > 0 {
		return strings.Join(lines, "|") + "|"
	}
	return "SDR — no color conversion needed"
}

func NeedsToneMapping(info *ColorSpaceInfo) bool {
	if info == nil {
		return false
	}
	if info.DolbyVision {
		return true
	}
	hasBT2020 := strings.Contains(info.ColorSpace, "bt2020") ||
		strings.Contains(info.Primaries, "bt2020")
	hasHDRTransfer := strings.Contains(info.Transfer, "smpte2084") ||
		strings.Contains(info.Transfer, "arib-std-b67") ||
		strings.Contains(info.Transfer, "arib")
	return hasBT2020 && hasHDRTransfer
}