//go:build !websocket

package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"minfo/internal/bdinfo"
	"minfo/internal/httpapi/transport"
)

var (
	jobManagerOnce sync.Once
	jobManager     *bdinfo.JobManager
	scanner        *bdinfo.Scanner
)

func initJobManager() {
	binPath := os.Getenv("BDINFO_BIN")
	if binPath == "" {
		binPath = "bdinfo"
	}

	tempDir := os.Getenv("BDINFO_TEMP_DIR")
	if tempDir == "" {
		tempDir = "/tmp/bdinfo-jobs"
	}

	var err error
	jobManager, err = bdinfo.NewJobManager(tempDir, binPath)
	if err != nil {
		return
	}

	scanner = bdinfo.NewScanner(jobManager)
	scanner.Start()
}

func getJobManager() *bdinfo.JobManager {
	jobManagerOnce.Do(initJobManager)
	return jobManager
}

func BDInfoListPlaylistsHandler(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := contextWithTimeout(r)
	defer cancel()

	result, err := bdinfo.ListPlaylists(ctx, path)
	if err != nil {
		transport.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	transport.WriteAnyJSON(w, http.StatusOK, result)
}

func BDInfoCreateJobHandler(w http.ResponseWriter, r *http.Request) {
	jm := getJobManager()
	if jm == nil {
		transport.WriteError(w, http.StatusInternalServerError, "job manager not initialized")
		return
	}

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

	scanMode := r.FormValue("scanMode")
	if scanMode == "" {
		scanMode = "auto"
	}

	var mpls []string
	if mplsStr := r.FormValue("playlists"); mplsStr != "" {
		mpls = strings.Split(mplsStr, ",")
		for i, m := range mpls {
			mpls[i] = strings.TrimSpace(m)
		}
	}

	if scanMode == "auto" {
		ctx, cancel := contextWithTimeout(r)
		defer cancel()

		result, err := bdinfo.ListPlaylists(ctx, path)
		if err != nil {
			transport.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if result.Recommendation != nil && len(result.Recommendation.Mpls) > 0 {
			mpls = result.Recommendation.Mpls
		}
	}

	job := jm.CreateJob(path, scanMode, mpls)

	transport.WriteAnyJSON(w, http.StatusOK, map[string]interface{}{
		"ok":  true,
		"job": job,
	})
}

func BDInfoListJobsHandler(w http.ResponseWriter, r *http.Request) {
	jm := getJobManager()
	if jm == nil {
		transport.WriteError(w, http.StatusInternalServerError, "job manager not initialized")
		return
	}

	jobs := jm.ListJobs()
	transport.WriteAnyJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"jobs": jobs,
	})
}

func BDInfoGetJobHandler(w http.ResponseWriter, r *http.Request) {
	jm := getJobManager()
	if jm == nil {
		transport.WriteError(w, http.StatusInternalServerError, "job manager not initialized")
		return
	}

	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		transport.WriteError(w, http.StatusBadRequest, "missing job id")
		return
	}

	job := jm.GetJob(jobID)
	if job == nil {
		transport.WriteError(w, http.StatusNotFound, "job not found")
		return
	}

	transport.WriteAnyJSON(w, http.StatusOK, map[string]interface{}{
		"ok":  true,
		"job": job,
	})
}

func BDInfoGetReportHandler(w http.ResponseWriter, r *http.Request) {
	jm := getJobManager()
	if jm == nil {
		transport.WriteError(w, http.StatusInternalServerError, "job manager not initialized")
		return
	}

	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		transport.WriteError(w, http.StatusBadRequest, "missing job id")
		return
	}

	job := jm.GetJob(jobID)
	if job == nil {
		transport.WriteError(w, http.StatusNotFound, "job not found")
		return
	}

	if job.Status != bdinfo.JobStatusDone {
		transport.WriteError(w, http.StatusBadRequest, "job not completed")
		return
	}

	if job.ReportPath == "" {
		transport.WriteError(w, http.StatusNotFound, "report not available")
		return
	}

	data, err := os.ReadFile(job.ReportPath)
	if err != nil {
		transport.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	transport.WriteAnyJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"report": string(data),
	})
}

func contextWithTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 5*time.Minute)
}
