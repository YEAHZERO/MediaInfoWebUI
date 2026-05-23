package handlers

import (
	"net/http"

	"mediainfo/internal/httpapi/transport"
	"mediainfo/internal/version"
)

var (
	BuildTime    = "unknown"
	BuildVersion = "1.5.6"
	BuildCommit  = "unknown"
)

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	// 使用 internal/version.Version 作为主版本号，BuildVersion 作为构建版本
	ver := version.Version
	if ver == "" || ver == "dev" {
		ver = BuildVersion
	}
	
	transport.WriteAnyJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"version": map[string]string{
			"buildTime": BuildTime,
			"version":   ver,
			"commit":    BuildCommit,
			"api":       "v1",
		},
	})
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	transport.WriteAnyJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"status": "running",
	})
}
