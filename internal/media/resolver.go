package media

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var errNoISO = errors.New("no iso found")
var errISOFound = errors.New("iso found")
var errNoVideo = errors.New("no video files found")

const mediaInfoCandidateLimit = 5

const MediaInfoCandidateLimit = mediaInfoCandidateLimit

type videoCandidate struct {
	path string
	size int64
}

func ResolveScreenshotSource(ctx context.Context, input string) (string, func(), error) {
	pt, _ := classifyPath(input)
	r := resolveResolver(pt)
	if r != nil {
		rc := ResolveContext{Context: ctx, Input: input, Type: pt}
		return r.ResolveScreenshot(rc)
	}

	info, err := os.Stat(input)
	if err != nil {
		return "", func() {}, err
	}

	if info.IsDir() {
		if bdmvPath := findBDMVInSubdirs(input); bdmvPath != "" {
			if _, ok := resolveBDMVRoot(bdmvPath); ok {
				m2ts, err := findLargestM2TS(bdmvPath)
				if err == nil {
					return m2ts, func() {}, nil
				}
			}
		}
		if _, m2tsPath, err := findBDMVInNestedDirs(input); err == nil && m2tsPath != "" {
			return m2tsPath, func() {}, nil
		}
		if isoPath := findISOInSubdirs(input); isoPath != "" {
			return resolveM2TSFromMountedISO(ctx, isoPath)
		}
		return findVideoFile(input), func() {}, nil
	}
	return input, func() {}, nil
}

func ResolveMediaInfoCandidates(ctx context.Context, input string, limit int) ([]string, func(), error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, func() {}, err
	}

	if !info.IsDir() {
		return []string{input}, func() {}, nil
	}

	pt, _ := classifyPath(input)
	r := resolveResolver(pt)
	if r != nil {
		rc := ResolveContext{Context: ctx, Input: input, Type: pt}
		return r.ResolveMediaInfo(rc, limit)
	}

	candidates, err := findVideoCandidates(input, limit)
	if err != nil {
		return nil, func() {}, err
	}
	return candidates, func() {}, nil
}

func ResolveBDInfoSource(ctx context.Context, input string) (string, func(), error) {
	pt, _ := classifyPath(input)
	r := resolveResolver(pt)
	if r != nil {
		rc := ResolveContext{Context: ctx, Input: input, Type: pt}
		return r.ResolveBDInfo(rc)
	}

	info, err := os.Stat(input)
	if err != nil {
		return "", func() {}, err
	}

	if !info.IsDir() {
		return "", func() {}, errors.New("path must be a folder containing BDMV or ISO")
	}

	if bdmvPath := findBDMVInSubdirs(input); bdmvPath != "" {
		if bdRoot, ok := resolveBDInfoRoot(bdmvPath); ok {
			return bdRoot, func() {}, nil
		}
	}

	bdmvPath, m2tsPath, err := findBDMVInNestedDirs(input)
	if err == nil && bdmvPath != "" {
		if m2tsPath != "" {
			return m2tsPath, func() {}, nil
		}
		return bdmvPath, func() {}, nil
	}

	if isoPath := findISOInSubdirs(input); isoPath != "" {
		return resolveBDInfoFromMountedISO(ctx, isoPath)
	}

	return "", func() {}, errors.New("path does not contain BDMV or BDISO content")
}

func resolveBDInfoRoot(path string) (string, bool) {
	base := filepath.Base(path)
	if strings.EqualFold(base, "BDMV") {
		return filepath.Dir(path), true
	}
	if strings.EqualFold(base, "STREAM") {
		return filepath.Dir(filepath.Dir(path)), true
	}
	bdmv := filepath.Join(path, "BDMV")
	if info, err := os.Stat(bdmv); err == nil && info.IsDir() {
		return path, true
	}
	return "", false
}

func resolveBDMVRoot(path string) (string, bool) {
	base := filepath.Base(path)
	if strings.EqualFold(base, "BDMV") || strings.EqualFold(base, "STREAM") {
		return path, true
	}
	bdmv := filepath.Join(path, "BDMV")
	if info, err := os.Stat(bdmv); err == nil && info.IsDir() {
		return bdmv, true
	}
	return "", false
}

func isISOFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".iso")
}

func findISOInDir(root string) (string, error) {
	var isoPath string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isISOFile(path) {
			isoPath = path
			return errISOFound
		}
		return nil
	})
	if err != nil && !errors.Is(err, errISOFound) {
		return "", err
	}
	if isoPath == "" {
		return "", errNoISO
	}
	return isoPath, nil
}

func findLargestM2TS(root string) (string, error) {
	searchRoot := root
	stream := filepath.Join(root, "STREAM")
	if info, err := os.Stat(stream); err == nil && info.IsDir() {
		searchRoot = stream
	}

	var largestPath string
	var largestSize int64
	err := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".m2ts") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > largestSize {
			largestSize = info.Size()
			largestPath = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if largestPath == "" {
		return "", errors.New("no m2ts files found under BDMV")
	}
	return largestPath, nil
}

func resolveM2TSFromMountedISO(ctx context.Context, isoPath string) (string, func(), error) {
	mountDir, cleanup, err := mountISO(ctx, isoPath)
	if err != nil {
		return "", func() {}, err
	}
	bdmvRoot, ok := resolveBDMVRoot(mountDir)
	if !ok {
		cleanup()
		return "", func() {}, errors.New("BDMV folder not found in ISO")
	}
	m2ts, err := findLargestM2TS(bdmvRoot)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	return m2ts, cleanup, nil
}

func resolveBDInfoFromMountedISO(ctx context.Context, isoPath string) (string, func(), error) {
	mountDir, cleanup, err := mountISO(ctx, isoPath)
	if err != nil {
		return "", func() {}, err
	}
	if _, ok := resolveBDInfoRoot(mountDir); !ok {
		cleanup()
		return "", func() {}, errors.New("BDMV folder not found in ISO")
	}
	return mountDir, cleanup, nil
}

func findVideoFile(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}

	var bestPath string
	var bestSize int64
	for _, entry := range entries {
		if entry.IsDir() || !isVideoFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() > bestSize {
			bestSize = info.Size()
			bestPath = filepath.Join(root, entry.Name())
		}
	}
	if bestPath != "" {
		return bestPath
	}
	path, _ := findLargestVideoFile(root)
	return path
}

func findVideoCandidates(root string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 1
	}

	items := make([]videoCandidate, 0, 16)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isVideoFile(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		items = append(items, videoCandidate{path: path, size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("%w under directory: %s", errNoVideo, root)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].size != items[j].size {
			return items[i].size > items[j].size
		}
		return items[i].path < items[j].path
	})
	if limit > len(items) {
		limit = len(items)
	}

	results := make([]string, 0, limit)
	for index := 0; index < limit; index++ {
		results = append(results, items[index].path)
	}
	return results, nil
}

func findLargestVideoFile(root string) (string, error) {
	var largestPath string
	var largestSize int64
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isVideoFile(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > largestSize {
			largestSize = info.Size()
			largestPath = path
		}
		return nil
	}); err != nil {
		return "", err
	}
	if largestPath == "" {
		return "", fmt.Errorf("%w under directory: %s", errNoVideo, root)
	}
	return largestPath, nil
}

func findBDMVInSubdirs(root string) string {
	var bdmvPath string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "BDMV") {
			bdmvPath = path
			return filepath.SkipAll
		}
		return nil
	})
	return bdmvPath
}

func findBDMVInNestedDirs(root string) (string, string, error) {
	var bestBDMVPath string
	var bestM2TSPath string
	var bestM2TSSize int64

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if !strings.EqualFold(d.Name(), "BDMV") {
			return nil
		}

		bdmvRoot := path
		streamDir := filepath.Join(path, "STREAM")
		if info, err := os.Stat(streamDir); err == nil && info.IsDir() {
			m2ts, m2tsSize, err := findLargestM2TSWithSize(streamDir)
			if err == nil && m2tsSize > bestM2TSSize {
				bestM2TSSize = m2tsSize
				bestM2TSPath = m2ts
				bestBDMVPath = bdmvRoot
			}
		}

		return nil
	})

	if err != nil {
		return "", "", err
	}

	if bestBDMVPath == "" {
		return "", "", fmt.Errorf("no BDMV directory found in nested structure")
	}

	return bestBDMVPath, bestM2TSPath, nil
}

func findLargestM2TSWithSize(root string) (string, int64, error) {
	var largestPath string
	var largestSize int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".m2ts") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > largestSize {
			largestSize = info.Size()
			largestPath = path
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	if largestPath == "" {
		return "", 0, fmt.Errorf("no m2ts files found")
	}
	return largestPath, largestSize, nil
}

func findISOInSubdirs(root string) string {
	var isoPath string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if isISOFile(path) {
			isoPath = path
			return filepath.SkipAll
		}
		return nil
	})
	return isoPath
}

func isVideoFileEntry(name string) bool {
	return isVideoExt(strings.ToLower(filepath.Ext(name)))
}

func isVideoFile(path string) bool {
	return isVideoFileEntry(path)
}
