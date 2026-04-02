package handlers

import (
	"net/http"

	"minfo/internal/httpapi/transport"
)

var (
	BuildTime    = "unknown"
	BuildVersion = "dev"
	BuildCommit  = "unknown"
)

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	transport.WriteAnyJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"version": map[string]string{
			"buildTime": BuildTime,
			"version":   BuildVersion,
			"commit":    BuildCommit,
			"api":       "v1",
		},
	})
}
