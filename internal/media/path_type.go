package media

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

type PathType int

const (
	PathTypeUnknown PathType = iota
	PathTypeFileVideo
	PathTypeFileISO
	PathTypeFileDVDTitle
	PathTypeOtherFile
	PathTypeDirBDMV
	PathTypeDirDVD
	PathTypeDirISO
	PathTypeDirVideo
)

type PathResolver interface {
	ResolveScreenshot(ctx ResolveContext) (string, func(), error)
	ResolveBDInfo(ctx ResolveContext) (string, func(), error)
	ResolveMediaInfo(ctx ResolveContext, limit int) ([]string, func(), error)
}

type ResolveContext struct {
	Context context.Context
	Input   string
	Type    PathType
}

func classifyPath(input string) (PathType, os.FileInfo) {
	info, err := os.Stat(input)
	if err != nil {
		return PathTypeUnknown, nil
	}
	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(input))
		if ext == ".iso" {
			return PathTypeFileISO, info
		}
		return PathTypeFileVideo, info
	}
	if hasBDMV(input) {
		return PathTypeDirBDMV, info
	}
	if hasDVDVideo(input) {
		return PathTypeDirDVD, info
	}
	if hasISOFile(input) {
		return PathTypeDirISO, info
	}
	if hasVideoFile(input) {
		return PathTypeDirVideo, info
	}
	return PathTypeUnknown, info
}

func isVideoExt(ext string) bool {
	switch ext {
	case ".m2ts", ".mts", ".mkv", ".mp4", ".m4v", ".mov", ".avi",
		".wmv", ".flv", ".mpg", ".mpeg", ".m2v", ".ts", ".vob", ".webm":
		return true
	}
	return false
}

func hasBDMV(dir string) bool {
	base := filepath.Base(dir)
	if strings.EqualFold(base, "BDMV") || strings.EqualFold(base, "STREAM") {
		return true
	}
	bdmv := filepath.Join(dir, "BDMV")
	if info, err := os.Stat(bdmv); err == nil && info.IsDir() {
		return true
	}
	return false
}

func hasDVDVideo(dir string) bool {
	base := filepath.Base(dir)
	if strings.EqualFold(base, "VIDEO_TS") {
		return true
	}
	video := filepath.Join(dir, "VIDEO_TS")
	if info, err := os.Stat(video); err == nil && info.IsDir() {
		return true
	}
	return false
}

func hasISOFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".iso") {
			return true
		}
	}
	return false
}

func hasVideoFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && isVideoExt(strings.ToLower(filepath.Ext(e.Name()))) {
			return true
		}
	}
	return false
}