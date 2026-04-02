package bdinfo

import (
	"bufio"
	"context"
	"encoding/json"
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

	tempDir, err := os.MkdirTemp("", "bdinfo-list-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "--list-json", bdPath)
	cmd.Dir = tempDir
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bdinfo list failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}

	var result struct {
		Playlists []Playlist `json:"playlists"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		playlists, parseErr := parsePlaylistListFromText(string(output))
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse playlist list: %v", parseErr)
		}
		result.Playlists = playlists
	}

	for i := range result.Playlists {
		if result.Playlists[i].LengthSeconds == 0 && result.Playlists[i].Length != "" {
			result.Playlists[i].LengthSeconds = parseLengthSeconds(result.Playlists[i].Length)
		}
	}

	sort.Slice(result.Playlists, func(i, j int) bool {
		return result.Playlists[i].LengthSeconds > result.Playlists[j].LengthSeconds
	})

	recommendation := RecommendPlaylists(result.Playlists)

	return &PlaylistListResult{
		Playlists:      result.Playlists,
		Recommendation: recommendation,
	}, nil
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
	re := regexp.MustCompile(`(\d{5})\.MPLS`)

	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); matches != nil {
			playlists = append(playlists, Playlist{
				Name: matches[1] + ".MPLS",
			})
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
