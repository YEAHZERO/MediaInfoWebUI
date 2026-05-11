package pixhost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"mediainfo/internal/config"
)

const apiURL = "https://api.pixhost.to/images"

var thumbHostPattern = regexp.MustCompile(`^t([0-9]+)\.pixhost\.to$`)

type apiResponse struct {
	ShowURL string `json:"show_url"`
	ThURL   string `json:"th_url"`
}

func endpoint() string {
	return config.Getenv("PIXHOST_API_URL", apiURL)
}

func uploadSingleImage(ctx context.Context, client *http.Client, apiURL, imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("img", filepath.Base(imagePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	if err := writer.WriteField("content_type", "0"); err != nil {
		return "", err
	}
	if err := writer.WriteField("max_th_size", "420"); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &body)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/json")
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
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("pixhost returned HTTP %d", response.StatusCode)
	}

	var payload apiResponse
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.ShowURL) == "" || strings.TrimSpace(payload.ThURL) == "" {
		return "", errors.New("pixhost response is missing show_url or th_url")
	}
	return normalizeDirectURL(payload.ThURL)
}

func normalizeDirectURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("pixhost direct URL is empty")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", err
	}

	parsed.Path = strings.Replace(parsed.Path, "/thumbs/", "/images/", 1)
	if matches := thumbHostPattern.FindStringSubmatch(strings.ToLower(parsed.Host)); len(matches) == 2 {
		parsed.Host = "img" + matches[1] + ".pixhost.to"
	}

	result := parsed.String()
	if !strings.HasPrefix(result, "http://") && !strings.HasPrefix(result, "https://") {
		return "", errors.New("pixhost direct URL is invalid")
	}
	return result, nil
}