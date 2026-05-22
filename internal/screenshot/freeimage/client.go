package freeimage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mediainfo/internal/config"
)

const apiURL = "https://freeimage.host/api/1/upload"

const defaultAPIKey = "6d207e02198a847aa98d0a2a901485a5"

type uploadResponse struct {
	StatusCode int    `json:"status_code"`
	StatusTxt  string `json:"status_txt"`
	Success    struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"success"`
	Image struct {
		URL       string `json:"url"`
		URLViewer string `json:"url_viewer"`
		Filename  string `json:"filename"`
		Name      string `json:"name"`
		Size      int    `json:"size"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
	} `json:"image"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func apiKey() string {
	return config.Getenv("FREEIMAGE_API_KEY", defaultAPIKey)
}

func endpoint() string {
	return config.Getenv("FREEIMAGE_API_URL", apiURL)
}

func UploadImage(ctx context.Context, client *http.Client, imagePath string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}

		directURL, err := doUpload(ctx, client, imagePath)
		if err == nil {
			return directURL, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("上传失败（重试 3 次后）: %w", lastErr)
}

func doUpload(ctx context.Context, client *http.Client, imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("key", apiKey()); err != nil {
		return "", err
	}
	if err := writer.WriteField("action", "upload"); err != nil {
		return "", err
	}
	if err := writer.WriteField("format", "json"); err != nil {
		return "", err
	}

	part, err := writer.CreateFormFile("source", filepath.Base(imagePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint(), &body)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	payloadBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	var payload uploadResponse
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		errMsg := fmt.Sprintf("freeimage.host returned HTTP %d", response.StatusCode)
		if payload.Error != nil && payload.Error.Message != "" {
			errMsg = fmt.Sprintf("freeimage.host error: %s", payload.Error.Message)
		}
		return "", errors.New(errMsg)
	}

	if payload.Image.URL == "" {
		return "", errors.New("freeimage.host response is missing image URL")
	}

	directURL := strings.TrimSpace(payload.Image.URL)
	if !strings.HasPrefix(directURL, "http://") && !strings.HasPrefix(directURL, "https://") {
		return "", errors.New("freeimage.host URL is invalid")
	}

	return directURL, nil
}