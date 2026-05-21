//go:build !websocket

package httpapi

import (
	"io/fs"
	"net/http"

	"mediainfo/internal/httpapi/handlers"
	"mediainfo/internal/httpapi/middleware"
)

func NewHandler(assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(assets)))
	mux.HandleFunc("/api/mediainfo", handlers.MediaInfoHandler("MEDIAINFO_BIN", "mediainfo"))
	mux.HandleFunc("/api/bdinfo", handlers.BDInfoHandler("BDINFO_BIN", "bdinfo"))
	mux.HandleFunc("/api/mkvmerge/tracks", handlers.MkvMergeTrackInfoHandler())
	mux.HandleFunc("/api/screenshots", handlers.ScreenshotsHandler)
	mux.HandleFunc("/api/path", handlers.PathSuggestHandler)
	mux.HandleFunc("/api/version", handlers.VersionHandler)
	mux.HandleFunc("/api/health", handlers.HealthHandler)

	mux.HandleFunc("/api/bdinfo/playlists", handlers.BDInfoListPlaylistsHandler)
	mux.HandleFunc("/api/bdinfo/jobs", handlers.BDInfoListJobsHandler)
	mux.HandleFunc("/api/bdinfo/job/create", handlers.BDInfoCreateJobHandler)
	mux.HandleFunc("/api/bdinfo/job", handlers.BDInfoGetJobHandler)
	mux.HandleFunc("/api/bdinfo/report", handlers.BDInfoGetReportHandler)
	mux.HandleFunc("/api/bdinfo/ws", handlers.BDInfoWebSocketStubHandler)

	mux.HandleFunc("/api/info-jobs", handlers.InfoJobsHandler)
	mux.HandleFunc("/api/info-jobs/", handlers.InfoJobHandler)
	mux.HandleFunc("/api/screenshot-jobs", handlers.ScreenshotJobsHandler)
	mux.HandleFunc("/api/screenshot-jobs/", handlers.ScreenshotJobHandler)

	return middleware.Logging(middleware.Authenticate(mux))
}
