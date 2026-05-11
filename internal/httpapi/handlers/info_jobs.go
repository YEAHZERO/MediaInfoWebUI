package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"mediainfo/internal/httpapi/transport"
)

func InfoJobsHandler(w http.ResponseWriter, r *http.Request) {
	if !transport.EnsurePost(w, r) {
		return
	}
	if err := transport.ParseForm(w, r); err != nil {
		writeInfoJobError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer transport.CleanupMultipart(r)

	kind := normalizeInfoJobKind(r.FormValue("kind"))
	if kind == "" {
		writeInfoJobError(w, http.StatusBadRequest, "invalid info job kind")
		return
	}

	inputPath, cleanup, err := transport.InputPath(r)
	if err != nil {
		writeInfoJobError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := createInfoJob(kind, inputPath, cleanup, r.FormValue("bdinfo_mode"))
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		writeInfoJobError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeInfoJobResponse(w, http.StatusAccepted, job.snapshot())
}

func InfoJobHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleInfoJobGet(w, r)
	case http.MethodDelete:
		handleInfoJobDelete(w, r)
	default:
		writeInfoJobError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleInfoJobGet(w http.ResponseWriter, r *http.Request) {
	jobID := parseInfoJobID(r)
	if jobID == "" || strings.Contains(jobID, "/") {
		writeInfoJobError(w, http.StatusNotFound, "job not found")
		return
	}

	job, ok := getInfoJob(jobID)
	if !ok {
		writeInfoJobError(w, http.StatusNotFound, "job not found")
		return
	}

	writeInfoJobResponse(w, http.StatusOK, job.snapshot())
}

func handleInfoJobDelete(w http.ResponseWriter, r *http.Request) {
	jobID := parseInfoJobID(r)
	if jobID == "" || strings.Contains(jobID, "/") {
		writeInfoJobError(w, http.StatusNotFound, "job not found")
		return
	}

	job, ok := getInfoJob(jobID)
	if !ok {
		writeInfoJobError(w, http.StatusNotFound, "job not found")
		return
	}

	job.requestCancel()
	writeInfoJobResponse(w, http.StatusOK, job.snapshot())
}

func parseInfoJobID(r *http.Request) string {
	jobID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/info-jobs/"))
	return strings.Trim(jobID, "/")
}

func normalizeInfoJobKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case infoKindMediaInfo:
		return infoKindMediaInfo
	case infoKindBDInfo:
		return infoKindBDInfo
	default:
		return ""
	}
}

func writeInfoJobResponse(w http.ResponseWriter, status int, payload transport.InfoJobResponse) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeInfoJobError(w http.ResponseWriter, status int, message string) {
	writeInfoJobResponse(w, status, transport.InfoJobResponse{
		OK:    false,
		Error: message,
	})
}