package handlers

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"minfo/internal/config"
	"minfo/internal/httpapi/transport"
	"minfo/internal/media"
	"minfo/internal/system"
)

func MediaInfoHandler(envKey, fallback string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !transport.EnsurePost(w, r) {
			return
		}
		if err := transport.ParseForm(w, r); err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer transport.CleanupMultipart(r)

		path, cleanup, err := transport.InputPath(r)
		if err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer cleanup()

		bin, err := system.ResolveBin(envKey, fallback)
		if err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), config.RequestTimeout)
		defer cancel()

		candidates, sourceCleanup, err := media.ResolveMediaInfoCandidates(ctx, path, media.MediaInfoCandidateLimit)
		if err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer sourceCleanup()

		var lastErr string
		for _, sourcePath := range candidates {
			sourceDir := filepath.Dir(sourcePath)
			sourceName := filepath.Base(sourcePath)
			stdout, stderr, err := system.RunCommandInDir(ctx, sourceDir, bin, sourceName)
			if err != nil {
				lastErr = system.BestErrorMessage(err, stderr, stdout)
				continue
			}

			output := system.CombineCommandOutput(stdout, stderr)
			if output == "" {
				lastErr = fmt.Sprintf("mediainfo returned empty output for: %s", sourcePath)
				continue
			}

			transport.WriteJSON(w, http.StatusOK, transport.InfoResponse{OK: true, Output: output})
			return
		}

		if lastErr == "" {
			lastErr = "mediainfo returned empty output"
		}
		transport.WriteError(w, http.StatusInternalServerError, lastErr)
	}
}

func BDInfoHandler(envKey, fallback string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !transport.EnsurePost(w, r) {
			return
		}
		if err := transport.ParseForm(w, r); err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer transport.CleanupMultipart(r)

		path, cleanup, err := transport.InputPath(r)
		if err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer cleanup()

		bin, err := system.ResolveBin(envKey, fallback)
		if err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), config.RequestTimeout)
		defer cancel()

		bdPath, bdCleanup, err := media.ResolveBDInfoSource(ctx, path)
		if err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer bdCleanup()

		stdout, stderr, err := system.RunCommand(ctx, bin, bdPath)
		if err != nil {
			transport.WriteError(w, http.StatusInternalServerError, system.BestErrorMessage(err, stderr, stdout))
			return
		}

		output := system.CombineCommandOutput(stdout, stderr)
		if shouldExtractBDInfoCode(r.FormValue("bdinfo_mode")) {
			output = extractBDInfoCodeBlock(output)
		}

		transport.WriteJSON(w, http.StatusOK, transport.InfoResponse{OK: true, Output: output})
	}
}

func shouldExtractBDInfoCode(mode string) bool {
	return strings.TrimSpace(strings.ToLower(mode)) != "full"
}

func extractBDInfoCodeBlock(output string) string {
	matches := regexp.MustCompile(`(?is)\[code\](.*?)\[/code\]`).FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return output
	}

	best := ""
	bestScore := -1
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		block := strings.TrimSpace(match[1])
		if block == "" {
			continue
		}

		score := len(block)
		if strings.Contains(strings.ToUpper(block), "DISC INFO:") {
			score += 1_000_000
		}

		if score > bestScore {
			best = block
			bestScore = score
		}
	}

	if best == "" {
		return output
	}
	return best
}

func MkvMergeTrackInfoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !transport.EnsurePost(w, r) {
			return
		}
		if err := transport.ParseForm(w, r); err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer transport.CleanupMultipart(r)

		path, cleanup, err := transport.InputPath(r)
		if err != nil {
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer cleanup()

		log.Printf("[mkvmerge] Input path: %s", path)

		bin, err := system.ResolveBin("MKVMERGE_BIN", "mkvmerge")
		if err != nil {
			log.Printf("[mkvmerge] Failed to resolve mkvmerge binary: %v", err)
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), config.RequestTimeout)
		defer cancel()

		// 优先使用 ResolveScreenshotSource 来处理 BDMV 目录和 ISO 文件，自动查找最大的 m2ts
		sourcePath, sourceCleanup, err := media.ResolveScreenshotSource(ctx, path)
		if err == nil {
			defer sourceCleanup()
			log.Printf("[mkvmerge] ResolveScreenshotSource returned: %s", sourcePath)
			stdout, stderr, err := system.RunCommand(ctx, bin, "-i", sourcePath)
			if err == nil {
				output := system.CombineCommandOutput(stdout, stderr)
				if output != "" {
					log.Printf("[mkvmerge] Success from ResolveScreenshotSource")
					transport.WriteJSON(w, http.StatusOK, transport.InfoResponse{OK: true, Output: output})
					return
				}
				log.Printf("[mkvmerge] Empty output from ResolveScreenshotSource")
			} else {
				log.Printf("[mkvmerge] RunCommand failed for ResolveScreenshotSource: %v", err)
			}
		} else {
			log.Printf("[mkvmerge] ResolveScreenshotSource failed: %v", err)
		}

		// 如果 ResolveScreenshotSource 失败，尝试在整个目录树中查找最大的 m2ts 文件
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			log.Printf("[mkvmerge] Path is a directory, searching for m2ts files")
			// 首先尝试查找最大的 m2ts 文件（类似 MediaInfo 的做法）
			m2tsPath, err := findLargestM2TSInTree(path)
			if err == nil && m2tsPath != "" {
				log.Printf("[mkvmerge] Found largest m2ts: %s", m2tsPath)
				stdout, stderr, err := system.RunCommand(ctx, bin, "-i", m2tsPath)
				if err == nil {
					output := system.CombineCommandOutput(stdout, stderr)
					if output != "" {
						log.Printf("[mkvmerge] Success from findLargestM2TSInTree")
						transport.WriteJSON(w, http.StatusOK, transport.InfoResponse{OK: true, Output: output})
						return
					}
					log.Printf("[mkvmerge] Empty output from findLargestM2TSInTree")
				} else {
					log.Printf("[mkvmerge] RunCommand failed for findLargestM2TSInTree: %v", err)
				}
			} else {
				log.Printf("[mkvmerge] findLargestM2TSInTree failed: %v", err)
			}
		}

		// 如果 ResolveScreenshotSource 失败，回退到 ResolveMediaInfoCandidates
		log.Printf("[mkvmerge] Falling back to ResolveMediaInfoCandidates")
		candidates, sourceCleanup, err := media.ResolveMediaInfoCandidates(ctx, path, media.MediaInfoCandidateLimit)
		if err != nil {
			log.Printf("[mkvmerge] ResolveMediaInfoCandidates failed: %v", err)
			transport.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer sourceCleanup()

		log.Printf("[mkvmerge] Found %d candidates", len(candidates))
		var lastErr string
		for _, sourcePath := range candidates {
			log.Printf("[mkvmerge] Trying candidate: %s", sourcePath)
			stdout, stderr, err := system.RunCommand(ctx, bin, "-i", sourcePath)
			if err != nil {
				lastErr = system.BestErrorMessage(err, stderr, stdout)
				log.Printf("[mkvmerge] RunCommand failed for candidate: %v", err)
				continue
			}

			output := system.CombineCommandOutput(stdout, stderr)
			if output == "" {
				lastErr = fmt.Sprintf("mkvmerge returned empty output for: %s", sourcePath)
				log.Printf("[mkvmerge] Empty output for candidate")
				continue
			}

			log.Printf("[mkvmerge] Success from candidate")
			transport.WriteJSON(w, http.StatusOK, transport.InfoResponse{OK: true, Output: output})
			return
		}

		if lastErr == "" {
			lastErr = "mkvmerge returned empty output"
		}
		log.Printf("[mkvmerge] All attempts failed: %s", lastErr)
		transport.WriteError(w, http.StatusInternalServerError, lastErr)
	}
}

// findBDMVInSubdirs 在子目录中递归查找 BDMV 目录
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

// findLargestM2TSInTree 在整个目录树中递归查找最大的 m2ts 文件（类似 MediaInfo 的做法）
func findLargestM2TSInTree(root string) (string, error) {
	var largestPath string
	var largestSize int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过错误，继续遍历
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".m2ts") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil // 跳过错误，继续遍历
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
		return "", fmt.Errorf("no m2ts files found in directory tree")
	}
	return largestPath, nil
}
