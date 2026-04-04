package bdinfo

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const minPlaylistDurationSeconds = 600

type Playlist struct {
	Name            string  `json:"name"`
	Length          string  `json:"length,omitempty"`
	LengthSeconds   float64 `json:"lengthSeconds,omitempty"`
	SizeBytes       int64   `json:"sizeBytes,omitempty"`
	ClipCount       int     `json:"clipCount,omitempty"`
	UniqueClipCount int     `json:"uniqueClipCount,omitempty"`
	Files           []string `json:"files,omitempty"`
}

type PlaylistListResult struct {
	Playlists      []Playlist `json:"playlists"`
	Recommendation *Selection `json:"recommendation,omitempty"`
}

type Selection struct {
	Mode             string   `json:"mode"`
	Mpls             []string `json:"mpls"`
	Reason           string   `json:"reason"`
	SelectedDuration float64  `json:"selectedDuration,omitempty"`
	SelectedSize     int64    `json:"selectedSize,omitempty"`
}

func ListPlaylists(ctx context.Context, bdPath string) (*PlaylistListResult, error) {
	bin := os.Getenv("BDINFO_BIN")
	if bin == "" {
		bin = "/usr/local/bin/bdinfo"
	}

	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("bdinfo binary not found: %v", err)
	}

	// 递归查找所有 BDMV 目录
	bdmvPaths := findAllBDMVPaths(bdPath)
	if len(bdmvPaths) == 0 {
		return nil, fmt.Errorf("no BDMV directory found in %s", bdPath)
	}

	allPlaylists := make([]Playlist, 0)
	
	for _, resolvedPath := range bdmvPaths {
		tempDir, err := os.MkdirTemp("", "bdinfo-list-*")
		if err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		
		// bdinfo 不支持 --list-json 参数，直接运行获取完整报告
		cmd := exec.CommandContext(ctx, bin, resolvedPath)
		cmd.Dir = tempDir
		output, err := cmd.Output()
		
		os.RemoveAll(tempDir)
		cancel()
		
		if err != nil {
			continue
		}

		// 从 bdinfo 输出中解析 playlist 信息
		playlists, err := parsePlaylistListFromText(string(output))
		if err != nil {
			continue
		}
		
		// 添加 BDMV 路径信息到 playlist 名称
		for i := range playlists {
			if playlists[i].LengthSeconds == 0 && playlists[i].Length != "" {
				playlists[i].LengthSeconds = parseLengthSeconds(playlists[i].Length)
			}
			// 添加路径前缀以区分不同 BDMV
			relPath := getRelativeBDMVPath(bdPath, resolvedPath)
			if relPath != "" && relPath != "." {
				playlists[i].Name = relPath + "/" + playlists[i].Name
			}
		}
		
		allPlaylists = append(allPlaylists, playlists...)
	}

	if len(allPlaylists) == 0 {
		return nil, fmt.Errorf("no playlists found")
	}

	sort.Slice(allPlaylists, func(i, j int) bool {
		return allPlaylists[i].LengthSeconds > allPlaylists[j].LengthSeconds
	})

	recommendation := RecommendPlaylists(allPlaylists)

	return &PlaylistListResult{
		Playlists:      allPlaylists,
		Recommendation: recommendation,
	}, nil
}

// findAllBDMVPaths 查找所有 BDMV 目录
// 采用 BDInfoWebUI 的策略：先向上查找，再向下搜索
func findAllBDMVPaths(root string) []string {
	var paths []string

	info, err := os.Stat(root)
	if err != nil {
		return paths
	}

	// 如果是文件（ISO），直接返回
	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(root), ".iso") {
			return []string{root}
		}
		return paths
	}

	// 1. 首先尝试向上查找 BDMV（处理传入路径是 BDMV 子目录的情况）
	bdmvPath := findBDMVByWalkingUp(root)
	if bdmvPath != "" {
		return []string{bdmvPath}
	}

	// 2. 然后向下搜索（处理传入路径是 BDMV 父目录的情况）
	paths = findBDMVInSubdirs(root, 0)

	return paths
}

// findBDMVByWalkingUp 向上遍历父目录查找 BDMV
func findBDMVByWalkingUp(startPath string) string {
	dir := startPath

	for dir != "" && dir != "/" && dir != "." {
		// 检查当前目录是否是 BDMV
		if strings.ToLower(filepath.Base(dir)) == "bdmv" {
			streamPath := filepath.Join(dir, "STREAM")
			if info, err := os.Stat(streamPath); err == nil && info.IsDir() {
				return dir
			}
		}

		// 检查当前目录下是否有 BDMV 子目录
		bdmvPath := filepath.Join(dir, "BDMV")
		if info, err := os.Stat(bdmvPath); err == nil && info.IsDir() {
			streamPath := filepath.Join(bdmvPath, "STREAM")
			if info, err := os.Stat(streamPath); err == nil && info.IsDir() {
				return bdmvPath
			}
		}

		// 向上到父目录
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// findBDMVInSubdirs 在子目录中查找 BDMV（限制搜索深度）
func findBDMVInSubdirs(root string, depth int) []string {
	var paths []string

	// 检查当前目录下是否有 BDMV 子目录
	bdmvPath := filepath.Join(root, "BDMV")
	if info, err := os.Stat(bdmvPath); err == nil && info.IsDir() {
		streamPath := filepath.Join(bdmvPath, "STREAM")
		if info, err := os.Stat(streamPath); err == nil && info.IsDir() {
			paths = append(paths, bdmvPath)
		}
	}

	// 限制递归深度，避免过度搜索
	if depth >= 2 {
		return paths
	}

	// 递归查找子目录
	entries, err := os.ReadDir(root)
	if err != nil {
		return paths
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		// 跳过特殊目录
		if name == "bdmv" || name == ".git" || name == "node_modules" || name == "any!" || name == "aacs" {
			continue
		}
		subPaths := findBDMVInSubdirs(filepath.Join(root, entry.Name()), depth+1)
		paths = append(paths, subPaths...)
	}

	return paths
}

// getRelativeBDMVPath 获取 BDMV 目录相对于根目录的相对路径
func getRelativeBDMVPath(root, bdmvPath string) string {
	rel, err := filepath.Rel(root, bdmvPath)
	if err != nil {
		return ""
	}
	// 如果 BDMV 是直接子目录，返回父目录名
	if strings.ToLower(filepath.Base(rel)) == "bdmv" {
		parent := filepath.Dir(rel)
		if parent == "." {
			return ""
		}
		return parent
	}
	return rel
}

func RecommendPlaylists(playlists []Playlist) *Selection {
	usable := make([]Playlist, 0)
	for _, p := range playlists {
		if p.LengthSeconds > 0 {
			usable = append(usable, p)
		}
	}

	filtered := make([]Playlist, 0)
	for _, p := range usable {
		if p.LengthSeconds >= minPlaylistDurationSeconds {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		return &Selection{
			Mode:   "whole",
			Mpls:   []string{},
			Reason: "no-playlists-over-10min",
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].SizeBytes > filtered[j].SizeBytes
	})

	largest := filtered[0]
	return &Selection{
		Mode:             "single",
		Mpls:             []string{largest.Name},
		Reason:           "largest-size-over-10min",
		SelectedDuration: largest.LengthSeconds,
		SelectedSize:     largest.SizeBytes,
	}
}

func parsePlaylistListFromText(text string) ([]Playlist, error) {
	playlists := make([]Playlist, 0)
	scanner := bufio.NewScanner(strings.NewReader(text))
	
	// 正则表达式匹配 playlist 信息
	playlistRe := regexp.MustCompile(`(?i)PLAYLIST\s*REPORT`)
	mplsRe := regexp.MustCompile(`(?i)(\d{5})\.MPLS`)
	lengthRe := regexp.MustCompile(`(?i)Length:\s*(\d{1,2}:\d{2}:\d{2})`)
	sizeRe := regexp.MustCompile(`(?i)Size:\s*([\d,]+)\s*bytes`)
	clipsRe := regexp.MustCompile(`(?i)Total\s*Video\s*Clips:\s*(\d+)`)
	
	var currentPlaylist *Playlist
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// 检查是否是新的 playlist 开始
		if playlistRe.MatchString(line) {
			if currentPlaylist != nil && currentPlaylist.Name != "" {
				playlists = append(playlists, *currentPlaylist)
			}
			currentPlaylist = &Playlist{}
			continue
		}
		
		if currentPlaylist == nil {
			continue
		}
		
		// 匹配 MPLS 文件名
		if matches := mplsRe.FindStringSubmatch(line); matches != nil {
			currentPlaylist.Name = matches[1] + ".MPLS"
			continue
		}
		
		// 匹配时长
		if matches := lengthRe.FindStringSubmatch(line); matches != nil {
			currentPlaylist.Length = matches[1]
			currentPlaylist.LengthSeconds = parseLengthSeconds(matches[1])
			continue
		}
		
		// 匹配大小
		if matches := sizeRe.FindStringSubmatch(line); matches != nil {
			sizeStr := strings.ReplaceAll(matches[1], ",", "")
			currentPlaylist.SizeBytes, _ = strconv.ParseInt(sizeStr, 10, 64)
			continue
		}
		
		// 匹配 clip 数量
		if matches := clipsRe.FindStringSubmatch(line); matches != nil {
			currentPlaylist.ClipCount, _ = strconv.Atoi(matches[1])
			continue
		}
	}
	
	// 添加最后一个 playlist
	if currentPlaylist != nil && currentPlaylist.Name != "" {
		playlists = append(playlists, *currentPlaylist)
	}
	
	// 如果没有解析到任何 playlist，尝试简单匹配 MPLS 文件名
	if len(playlists) == 0 {
		scanner = bufio.NewScanner(strings.NewReader(text))
		mplsReSimple := regexp.MustCompile(`(\d{5})\.MPLS`)
		seen := make(map[string]bool)
		
		for scanner.Scan() {
			line := scanner.Text()
			if matches := mplsReSimple.FindStringSubmatch(line); matches != nil {
				name := matches[1] + ".MPLS"
				if !seen[name] {
					seen[name] = true
					playlists = append(playlists, Playlist{Name: name})
				}
			}
		}
	}

	return playlists, nil
}

func parseLengthSeconds(length string) float64 {
	parts := strings.Split(length, ":")
	if len(parts) < 3 {
		return 0
	}

	hours, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	minutes, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	seconds, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)

	return hours*3600 + minutes*60 + seconds
}

func ResolveBDPath(inputPath string) (string, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		if strings.HasSuffix(strings.ToLower(inputPath), ".iso") {
			return inputPath, nil
		}
		return "", fmt.Errorf("not a Blu-ray directory or ISO")
	}

	bdmvPath := filepath.Join(inputPath, "BDMV")
	if _, err := os.Stat(bdmvPath); err == nil {
		return bdmvPath, nil
	}

	if strings.ToLower(filepath.Base(inputPath)) == "bdmv" {
		streamPath := filepath.Join(inputPath, "STREAM")
		if _, err := os.Stat(streamPath); err == nil {
			return inputPath, nil
		}
	}

	return "", fmt.Errorf("no BDMV directory found")
}
