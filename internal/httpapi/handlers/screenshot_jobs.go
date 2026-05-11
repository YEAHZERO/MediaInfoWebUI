package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"mediainfo/internal/httpapi/transport"
)

func ScreenshotJobsHandler(w http.ResponseWriter, r *http.Request) {
	if !transport.EnsurePost(w, r) {
		return
	}
	if err := transport.ParseForm(w, r); err != nil {
		writeScreenshotJobError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer transport.CleanupMultipart(r)

	request, err := parseScreenshotFormRequest(r)
	if err != nil {
		writeScreenshotJobError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := createScreenshotJob(*request)
	if err != nil {
		if request.Cleanup != nil {
			request.Cleanup()
		}
		writeScreenshotJobError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeScreenshotJobResponse(w, http.StatusAccepted, job.snapshot())
}

func ScreenshotJobHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleScreenshotJobGet(w, r)
	case http.MethodDelete:
		handleScreenshotJobDelete(w, r)
	default:
		writeScreenshotJobError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleScreenshotJobGet(w http.ResponseWriter, r *http.Request) {
	jobID := parseScreenshotJobID(r)
	if jobID == "" || strings.Contains(jobID, "/") {
		writeScreenshotJobError(w, http.StatusNotFound, "job not found")
		return
	}

	job, ok := getScreenshotJob(jobID)
	if !ok {
		writeScreenshotJobError(w, http.StatusNotFound, "job not found")
		return
	}

	writeScreenshotJobResponse(w, http.StatusOK, job.snapshot())
}

func handleScreenshotJobDelete(w http.ResponseWriter, r *http.Request) {
	jobID := parseScreenshotJobID(r)
	if jobID == "" || strings.Contains(jobID, "/") {
		writeScreenshotJobError(w, http.StatusNotFound, "job not found")
		return
	}

	job, ok := getScreenshotJob(jobID)
	if !ok {
		writeScreenshotJobError(w, http.StatusNotFound, "job not found")
		return
	}

	job.requestCancel()
	writeScreenshotJobResponse(w, http.StatusOK, job.snapshot())
}

func parseScreenshotJobID(r *http.Request) string {
	jobID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/screenshot-jobs/"))
	return strings.Trim(jobID, "/")
}

func writeScreenshotJobResponse(w http.ResponseWriter, status int, payload transport.ScreenshotJobResponse) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeScreenshotJobError(w http.ResponseWriter, status int, message string) {
	writeScreenshotJobResponse(w, status, transport.ScreenshotJobResponse{
		OK:    false,
		Error: message,
	})
}