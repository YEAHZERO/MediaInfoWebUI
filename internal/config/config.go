package config

import (
	"log"
	"os"
	"strings"
	"time"
)

const (
	DefaultPort           = "28888"
	DefaultRoot           = "/media"
	MaxUploadBytes        = int64(8 << 30)
	MaxMemoryBytes        = int64(32 << 20)
	MaxSuggestions        = 200
	MountTimeout          = 30 * time.Second
	UmountTimeout         = 30 * time.Second
	DefaultRequestTimeout = 10 * time.Minute

	DefaultScreenshotCompressThreshold = int64(10 * 1024 * 1024)
	DefaultScreenshotCompressStrategy  = "auto"
)

var RequestTimeout = DurationFromEnv("REQUEST_TIMEOUT", DefaultRequestTimeout)

var FFmpegSSECompat = BoolFromEnv("FFMPEG_SSE_COMPAT", false)

func Getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func DurationFromEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		log.Printf("invalid %s=%q; fallback to %s", key, value, fallback)
		return fallback
	}
	return duration
}

func BoolFromEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		log.Printf("invalid %s=%q; fallback to %t", key, value, fallback)
		return fallback
	}
}
