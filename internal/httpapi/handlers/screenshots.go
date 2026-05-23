package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mediainfo/internal/config"
	"mediainfo/internal/httpapi/transport"
	"mediainfo/internal/screenshot"
)

func ScreenshotsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleScreenshotZipDownload(w, r)
	case http.MethodHead:
		handleScreenshotZipDownload(w, r)
	case http.MethodPost:
		handleScreenshotsPost(w, r)
	default:
		transport.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleScreenshotsPost(w http.ResponseWriter, r *http.Request) {
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

	mode := screenshot.NormalizeMode(r.FormValue("mode"))
	variant := screenshot.NormalizeVariant(r.FormValue("variant"))
	subtitleMode := screenshot.NormalizeSubtitleMode(r.FormValue("subtitle_mode"))
	count := screenshot.NormalizeCount(r.FormValue("count"))

	ctx, cancel := context.WithTimeout(r.Context(), config.RequestTimeout)
	defer cancel()

	tempDir, err := os.MkdirTemp("", "mediainfo-shots-*")
	if err != nil {
		transport.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(tempDir)

	if mode == screenshot.ModeLinks {
		hostName := r.FormValue("host")
		result, err := screenshot.RunUploadWithLogs(ctx, path, tempDir, variant, subtitleMode, hostName, count)
		if err != nil {
			transport.WriteJSON(w, http.StatusInternalServerError, transport.InfoResponse{OK: false, Error: err.Error(), Logs: result.Logs})
			return
		}
		transport.WriteJSON(w, http.StatusOK, transport.InfoResponse{OK: true, Output: result.Output, Logs: result.Logs})
		return
	}

	if shouldPrepareDownload(r) {
		downloadURL, logs, files, err := prepareScreenshotZipDownload(ctx, path, tempDir, variant, subtitleMode, count)
		if err != nil {
			transport.WriteJSON(w, http.StatusInternalServerError, transport.InfoResponse{OK: false, Error: err.Error(), Logs: logs})
			return
		}
		resp := transport.ScreenshotResponse{
			OK:     true,
			Output: downloadURL,
			Logs:   logs,
			Files:  files,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	if err := writeScreenshotZipResponse(ctx, w, path, tempDir, variant, subtitleMode, count); err != nil {
		transport.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}

func handleScreenshotZipDownload(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token != "" {
		servePreparedScreenshotDownload(w, r, token)
		return
	}

	path, err := inputPathFromQuery(r)
	if err != nil {
		transport.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	variant := screenshot.NormalizeVariant(r.URL.Query().Get("variant"))
	subtitleMode := screenshot.NormalizeSubtitleMode(r.URL.Query().Get("subtitle_mode"))
	count := screenshot.NormalizeCount(r.URL.Query().Get("count"))

	ctx, cancel := context.WithTimeout(r.Context(), config.RequestTimeout)
	defer cancel()

	tempDir, err := os.MkdirTemp("", "mediainfo-shots-*")
	if err != nil {
		transport.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(tempDir)

	if err := writeScreenshotZipResponse(ctx, w, path, tempDir, variant, subtitleMode, count); err != nil {
		transport.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}

func shouldPrepareDownload(r *http.Request) bool {
	return strings.TrimSpace(r.FormValue("prepare_download")) == "1"
}

func prepareScreenshotZipDownload(ctx context.Context, path, tempDir, variant, subtitleMode string, count int, logHandlers ...screenshot.LogHandler) (string, string, []transport.ScreenshotFileInfo, error) {
	zipBytes, logs, files, err := generateScreenshotZip(ctx, path, tempDir, variant, subtitleMode, count, logHandlers...)
	if err != nil {
		return "", logs, nil, err
	}

	token, err := screenshot.SavePreparedDownload(zipBytes)
	if err != nil {
		return "", logs, nil, err
	}
	return "/api/screenshots?token=" + token, logs, files, nil
}

func writeScreenshotZipResponse(ctx context.Context, w http.ResponseWriter, path, tempDir, variant, subtitleMode string, count int) error {
	zipBytes, _, _, err := generateScreenshotZip(ctx, path, tempDir, variant, subtitleMode, count)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"screenshots.zip\"")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(zipBytes); err != nil {
		log.Printf("write response: %v", err)
	}
	return nil
}

func generateScreenshotZip(ctx context.Context, path, tempDir, variant, subtitleMode string, count int, logHandlers ...screenshot.LogHandler) ([]byte, string, []transport.ScreenshotFileInfo, error) {
	result, err := screenshot.RunScriptWithLogs(ctx, path, tempDir, variant, subtitleMode, count, logHandlers...)
	if err != nil {
		return nil, result.Logs, nil, err
	}

	files := make([]transport.ScreenshotFileInfo, 0, len(result.Files))
	for _, f := range result.Files {
		files = append(files, transport.ScreenshotFileInfo{
			Path: f.Path,
			Name: f.Name,
			Size: f.Size,
		})
	}

	engineFiles := make([]screenshot.ScreenshotFileInfo, 0, len(result.Files))
	for _, f := range result.Files {
		engineFiles = append(engineFiles, screenshot.ScreenshotFileInfo{
			Path: f.Path,
			Name: f.Name,
			Size: f.Size,
		})
	}

	zipBytes, err := screenshot.ZipFiles(engineFiles)
	if err != nil {
		return nil, result.Logs, nil, err
	}
	return zipBytes, result.Logs, files, nil
}

func servePreparedScreenshotDownload(w http.ResponseWriter, r *http.Request, token string) {
	filePath, err := screenshot.GetPreparedDownload(token)
	if err != nil {
		transport.WriteError(w, http.StatusNotFound, "download expired or not found")
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"screenshots.zip\"")
	http.ServeFile(w, r, filePath)
}

func inputPathFromQuery(r *http.Request) (string, error) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	path = strings.Trim(path, "\"")
	if path == "" {
		return "", fmt.Errorf("missing path")
	}
	path = filepath.Clean(path)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("path not found: %v", err)
	}
	return path, nil
}
