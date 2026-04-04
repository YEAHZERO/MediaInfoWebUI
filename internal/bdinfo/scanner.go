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

	"minfo/internal/media"
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

	args := s.buildArgs(job, bdPath, jobDir)

	cmd := exec.CommandContext(ctx, s.jm.BinPath(), args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("failed to start bdinfo: %v", err)
		job.EndTime = time.Now()
		s.jm.UpdateJob(job)
		return
	}

	if err := cmd.Wait(); err != nil {
		job.Status = JobStatusError
		job.Error = fmt.Sprintf("bdinfo failed: %v", err)
		job.EndTime = time.Now()
		s.jm.UpdateJob(job)
		return
	}

	reportPath := filepath.Join(jobDir, "report.txt")
	reportContent := stdout.String() + stderr.String()
	if err := os.WriteFile(reportPath, []byte(reportContent), 0644); err == nil {
		job.ReportPath = reportPath
		job.Summary = s.extractSummary(reportContent)
	}

	job.Status = JobStatusDone
	job.Progress = 100
	job.EndTime = time.Now()
	s.jm.UpdateJob(job)
}

func (s *Scanner) buildArgs(job *Job, bdPath, jobDir string) []string {
	args := []string{bdPath}

	switch job.ScanMode {
	case "whole":
		args = append(args, "--whole")
	case "playlists":
		if len(job.SelectedMpls) > 0 {
			args = append(args, "--playlists")
			args = append(args, strings.Join(job.SelectedMpls, ","))
		}
	case "auto":
		if len(job.SelectedMpls) > 0 {
			args = append(args, "--playlists")
			args = append(args, strings.Join(job.SelectedMpls, ","))
		}
	default:
		if len(job.SelectedMpls) > 0 {
			args = append(args, "--playlists")
			args = append(args, strings.Join(job.SelectedMpls, ","))
		}
	}

	return args
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
