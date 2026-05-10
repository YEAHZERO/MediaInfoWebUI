package bdinfo

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mediainfo/internal/media"
	"mediainfo/internal/system"
)

type Scanner struct {
	jm       *JobManager
	mu       sync.Mutex
	running  bool
	cancelFn context.CancelFunc
}

func NewScanner(jm *JobManager) *Scanner {
	return &Scanner{jm: jm}
}

func (s *Scanner) Start() {
	go s.runLoop()
}

func (s *Scanner) runLoop() {
	for {
		job := s.jm.StartNext()
		if job == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		s.runJob(job)
		s.jm.FinishActive()
	}
}

func (s *Scanner) runJob(job *Job) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running = true
	s.cancelFn = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.cancelFn = nil
		s.mu.Unlock()
	}()

	jobDir := s.jm.JobDir(job.ID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("failed to create job directory: %v", err)
		job.EndTime = time.Now()
		s.jm.UpdateJob(job)
		return
	}

	bdPath, bdCleanup, err := media.ResolveBDInfoSource(ctx, job.Path)
	if err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("failed to resolve BD path: %v", err)
		job.EndTime = time.Now()
		s.jm.UpdateJob(job)
		return
	}
	defer bdCleanup()

	args := buildBDInfoArgs(job, bdPath, jobDir)

	stdout, stderr, err := system.RunCommandLive(ctx, s.jm.BinPath(), func(stream, line string) {
		s.parseProgressLine(line, job)
	}, args...)

	if err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("bdinfo failed: %v", system.BestErrorMessage(err, stderr, stdout))
		job.EndTime = time.Now()
		s.jm.UpdateJob(job)
		return
	}

	reportPath := filepath.Join(jobDir, "report.txt")
	reportContent := stdout + stderr
	if err := os.WriteFile(reportPath, []byte(reportContent), 0644); err == nil {
		job.ReportPath = reportPath
		job.Summary = SelectLargestPlaylistBlock(reportContent)
		if job.Summary == "" {
			job.Summary = s.extractSummary(reportContent)
		}
	}

	job.Status = JobStatusDone
	job.Progress = 100
	job.EndTime = time.Now()
	s.jm.UpdateJob(job)
}

func buildBDInfoArgs(job *Job, bdPath, jobDir string) []string {
	return composeArgs(
		baseArgs(bdPath),
		scanModeArgs(job.ScanMode, job.SelectedMpls),
		outputArgs(jobDir),
	)
}

func composeArgs(parts ...[]string) []string {
	var total int
	for _, p := range parts {
		total += len(p)
	}
	result := make([]string, 0, total)
	for _, p := range parts {
		result = append(result, p...)
	}
	return result
}

func baseArgs(bdPath string) []string {
	return []string{bdPath}
}

func scanModeArgs(scanMode string, selectedMpls []string) []string {
	switch scanMode {
	case "whole":
		return []string{"--whole"}
	case "playlists", "auto":
		if len(selectedMpls) > 0 {
			return []string{"--playlists", strings.Join(selectedMpls, ",")}
		}
	default:
		if len(selectedMpls) > 0 {
			return []string{"--playlists", strings.Join(selectedMpls, ",")}
		}
	}
	return nil
}

func outputArgs(jobDir string) []string {
	return []string{"--output", jobDir}
}

func (s *Scanner) parseProgress(reader io.Reader, job *Job) {
	scanner := bufio.NewScanner(reader)
	progressRe := regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*%`)
	etaRe := regexp.MustCompile(`(?i)ETA[:\s]+(\d+):(\d+):(\d+)`)

	for scanner.Scan() {
		line := scanner.Text()

		if matches := progressRe.FindStringSubmatch(line); matches != nil {
			if progress, err := strconv.ParseFloat(matches[1], 64); err == nil {
				job.Progress = progress
				s.jm.UpdateJob(job)
			}
		}

		if matches := etaRe.FindStringSubmatch(line); matches != nil {
			hours, _ := strconv.Atoi(matches[1])
			mins, _ := strconv.Atoi(matches[2])
			secs, _ := strconv.Atoi(matches[3])
			job.ETASec = hours*3600 + mins*60 + secs
			s.jm.UpdateJob(job)
		}
	}
}

func (s *Scanner) extractSummary(report string) string {
	sections := []string{
		"DISC INFO",
		"VIDEO",
		"AUDIO",
		"SUBTITLES",
	}

	var summary strings.Builder
	lines := strings.Split(report, "\n")
	currentSection := ""
	inSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		for _, section := range sections {
			if strings.Contains(strings.ToUpper(trimmed), section) {
				if inSection && currentSection != section {
					summary.WriteString("\n")
				}
				currentSection = section
				inSection = true
				break
			}
		}

		if inSection && trimmed != "" {
			summary.WriteString(line + "\n")
		}
	}

	return summary.String()
}

func (s *Scanner) parseProgressLine(line string, job *Job) {
	progressRe := regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*%`)
	etaRe := regexp.MustCompile(`(?i)ETA[:\s]+(\d+):(\d+):(\d+)`)

	if matches := progressRe.FindStringSubmatch(line); matches != nil {
		if progress, err := strconv.ParseFloat(matches[1], 64); err == nil {
			job.Progress = progress
			s.jm.UpdateJob(job)
		}
	}

	if matches := etaRe.FindStringSubmatch(line); matches != nil {
		hours, _ := strconv.Atoi(matches[1])
		mins, _ := strconv.Atoi(matches[2])
		secs, _ := strconv.Atoi(matches[3])
		job.ETASec = hours*3600 + mins*60 + secs
		s.jm.UpdateJob(job)
	}
}

func SelectLargestPlaylistBlock(report string) string {
	lines := strings.Split(report, "\n")
	if len(lines) == 0 {
		return ""
	}

	type playlistBlock struct {
		start int
		end   int
		size  int
	}

	var blocks []playlistBlock
	var currentStart = -1
	playlistRe := regexp.MustCompile(`(?i)Playlist\s+\d+`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if playlistRe.MatchString(trimmed) {
			if currentStart >= 0 {
				blocks = append(blocks, playlistBlock{
					start: currentStart,
					end:   i - 1,
					size:  i - currentStart,
				})
			}
			currentStart = i
		}
	}

	if currentStart >= 0 {
		blocks = append(blocks, playlistBlock{
			start: currentStart,
			end:   len(lines) - 1,
			size:  len(lines) - currentStart,
		})
	}

	if len(blocks) == 0 {
		return ""
	}

	largest := blocks[0]
	for _, block := range blocks[1:] {
		if block.size > largest.size {
			largest = block
		}
	}

	var result strings.Builder
	for i := largest.start; i <= largest.end && i < len(lines); i++ {
		result.WriteString(lines[i])
		result.WriteString("\n")
	}

	return result.String()
}

func (s *Scanner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
}

func (s *Scanner) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
