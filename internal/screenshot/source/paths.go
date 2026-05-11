package source

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mediainfo/internal/screenshot/dvdinfo"
)

func LooksLikeDVDSource(path string) bool {
	return dvdinfo.LooksLikeDVDSource(path)
}

func FindBlurayRootFromVideo(videoPath string) (string, bool) {
	dir := filepath.Dir(videoPath)
	for candidate := dir; candidate != "/" && len(candidate) > 1; candidate = filepath.Dir(candidate) {
		if isBDMVRoot(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func isBDMVRoot(path string) bool {
	info, err := os.Stat(filepath.Join(path, "BDMV", "index.bdmv"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

type playlistScore struct {
	path   string
	clipID string
	count  int
}

func ListBlurayPlaylistsRanked(root, clip string) []string {
	playlistDir := filepath.Join(root, "BDMV", "PLAYLIST")
	entries, err := os.ReadDir(playlistDir)
	if err != nil {
		return nil
	}

	mplsClips := extractMPLSClipIDs(root, clip)

	var scored []playlistScore
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToUpper(entry.Name()), ".MPLS") {
			continue
		}
		path := filepath.Join(playlistDir, entry.Name())
		clips := extractSingleMPLSClipIDs(path)
		count := 0
		for _, target := range mplsClips {
			for _, clip := range clips {
				if target == clip {
					count++
				}
			}
		}
		scored = append(scored, playlistScore{path: path, clipID: entry.Name(), count: count})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].count == scored[j].count {
			return scored[i].clipID > scored[j].clipID
		}
		return scored[i].count > scored[j].count
	})

	result := make([]string, 0, len(scored))
	for _, s := range scored {
		result = append(result, s.path)
	}
	return result
}

func extractMPLSClipIDs(root, clip string) []string {
	clipID := strings.TrimSuffix(clip, ".m2ts")
	playlistDir := filepath.Join(root, "BDMV", "PLAYLIST")
	entries, err := os.ReadDir(playlistDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToUpper(entry.Name()), ".MPLS") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(playlistDir, entry.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, clipID) {
			return extractSingleMPLSClipIDs(filepath.Join(playlistDir, entry.Name()))
		}
	}
	return nil
}

func extractSingleMPLSClipIDs(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)

	var ids []string
	seen := make(map[string]bool)
	idx := 0
	for {
		pos := strings.Index(content[idx:], ".m2ts")
		if pos < 0 {
			break
		}
		start := idx + pos - 5
		if start < 0 {
			start = 0
		}
		end := idx + pos + 5
		if end > len(content) {
			end = len(content)
		}
		candidate := content[start:end]
		cleaned := strings.TrimSpace(candidate)
		if !seen[cleaned] {
			seen[cleaned] = true
			ids = append(ids, cleaned)
		}
		idx = idx + pos + 5
	}
	return ids
}