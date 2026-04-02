package bdinfo

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxJobs = 100

type JobStatus string

const (
	JobStatusQueued  JobStatus = "queued"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusError   JobStatus = "error"
)

type Job struct {
	ID             string      `json:"id"`
	Path           string      `json:"path"`
	Status         JobStatus   `json:"status"`
	Progress       float64     `json:"progress"`
	ETASec         int         `json:"etaSec,omitempty"`
	StartTime      time.Time   `json:"startTime,omitempty"`
	EndTime        time.Time   `json:"endTime,omitempty"`
	ReportPath     string      `json:"reportPath,omitempty"`
	Error          string      `json:"error,omitempty"`
	Selection      *Selection  `json:"selection,omitempty"`
	Summary        string      `json:"summary,omitempty"`
	ReportData     interface{} `json:"reportData,omitempty"`
	ScanMode       string      `json:"scanMode,omitempty"`
	SelectedMpls   []string    `json:"selectedMpls,omitempty"`
}

type JobManager struct {
	mu       sync.RWMutex
	jobs     map[string]*Job
	order    []string
	queue    []string
	active   string
	tempDir  string
	binPath  string
	wsHub    *WebSocketHub
}

func NewJobManager(tempDir, binPath string) (*JobManager, error) {
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	return &JobManager{
		jobs:    make(map[string]*Job),
		order:   make([]string, 0, maxJobs),
		queue:   make([]string, 0, maxJobs),
		tempDir: tempDir,
		binPath: binPath,
		wsHub:   NewWebSocketHub(),
	}, nil
}

func (jm *JobManager) WebSocketHub() *WebSocketHub {
	return jm.wsHub
}

func (jm *JobManager) CreateJob(path string, scanMode string, mpls []string) *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if len(jm.order) >= maxJobs {
		oldest := jm.order[len(jm.order)-1]
		delete(jm.jobs, oldest)
		jm.order = jm.order[:len(jm.order)-1]
	}

	job := &Job{
		ID:           generateJobID(),
		Path:         path,
		Status:       JobStatusQueued,
		Progress:     0,
		StartTime:    time.Now(),
		ScanMode:     scanMode,
		SelectedMpls: mpls,
	}

	jm.jobs[job.ID] = job
	jm.order = append([]string{job.ID}, jm.order...)
	jm.queue = append(jm.queue, job.ID)

	jm.broadcastUpdate(job)

	return job
}

func (jm *JobManager) GetJob(id string) *Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.jobs[id]
}

func (jm *JobManager) ListJobs() []*Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	jobs := make([]*Job, 0, len(jm.order))
	for _, id := range jm.order {
		if job, ok := jm.jobs[id]; ok {
			jobs = append(jobs, job)
		}
	}
	return jobs
}

func (jm *JobManager) UpdateJob(job *Job) {
	jm.mu.Lock()
	jm.jobs[job.ID] = job
	jm.mu.Unlock()
	jm.broadcastUpdate(job)
}

func (jm *JobManager) StartNext() *Job {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	if jm.active != "" {
		return nil
	}

	if len(jm.queue) == 0 {
		return nil
	}

	nextID := jm.queue[0]
	jm.queue = jm.queue[1:]
	jm.active = nextID

	job := jm.jobs[nextID]
	job.Status = JobStatusRunning
	jm.broadcastUpdate(job)

	return job
}

func (jm *JobManager) FinishActive() {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.active = ""
}

func (jm *JobManager) JobDir(jobID string) string {
	return filepath.Join(jm.tempDir, jobID)
}

func (jm *JobManager) BinPath() string {
	return jm.binPath
}

func (jm *JobManager) broadcastUpdate(job *Job) {
	if jm.wsHub != nil {
		jm.wsHub.BroadcastJobUpdate(job)
	}
}

func generateJobID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
