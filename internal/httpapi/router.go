//go:build websocket

package httpapi

import (
	"io/fs"
	"net/http"

	"minfo/internal/httpapi/handlers"
	"minfo/internal/httpapi/middleware"
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

	mux.HandleFunc("/api/bdinfo/playlists", handlers.BDInfoListPlaylistsHandler)
	mux.HandleFunc("/api/bdinfo/jobs", handlers.BDInfoListJobsHandler)
	mux.HandleFunc("/api/bdinfo/job/create", handlers.BDInfoCreateJobHandler)
	mux.HandleFunc("/api/bdinfo/job", handlers.BDInfoGetJobHandler)
	mux.HandleFunc("/api/bdinfo/report", handlers.BDInfoGetReportHandler)
	mux.HandleFunc("/api/bdinfo/ws", handlers.BDInfoWebSocketHandler)

	return middleware.Logging(middleware.Authenticate(mux))
}
