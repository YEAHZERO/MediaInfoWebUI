package transport

type LogEntry struct {
	Timestamp string `json:"timestamp,omitempty"`
	Message   string `json:"message,omitempty"`
}

type ImageLinkItem struct {
	URL      string `json:"url,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

type TaskProgress struct {
	Percent       float64 `json:"percent,omitempty"`
	Stage         string  `json:"stage,omitempty"`
	Detail        string  `json:"detail,omitempty"`
	Current       int     `json:"current,omitempty"`
	Total         int     `json:"total,omitempty"`
	Indeterminate bool    `json:"indeterminate,omitempty"`
}

type InfoResponse struct {
	OK              bool            `json:"ok"`
	Output          string          `json:"output,omitempty"`
	Error           string          `json:"error,omitempty"`
	Logs            string          `json:"logs,omitempty"`
	LogEntries      []LogEntry      `json:"log_entries,omitempty"`
	LinkItems       []ImageLinkItem `json:"link_items,omitempty"`
	PNGLossyFiles   []string        `json:"png_lossy_files,omitempty"`
	PNGLossyIndexes []int           `json:"png_lossy_indexes,omitempty"`
}

type ScreenshotJobResponse struct {
	OK              bool            `json:"ok"`
	JobID           string          `json:"job_id,omitempty"`
	Status          string          `json:"status,omitempty"`
	Mode            string          `json:"mode,omitempty"`
	Output          string          `json:"output,omitempty"`
	DownloadURL     string          `json:"download_url,omitempty"`
	Error           string          `json:"error,omitempty"`
	Logs            string          `json:"logs,omitempty"`
	LogEntries      []LogEntry      `json:"log_entries,omitempty"`
	Progress        *TaskProgress   `json:"progress,omitempty"`
	LinkItems       []ImageLinkItem `json:"link_items,omitempty"`
	PNGLossyFiles   []string        `json:"png_lossy_files,omitempty"`
	PNGLossyIndexes []int           `json:"png_lossy_indexes,omitempty"`
	Filename        string          `json:"filename,omitempty"`
	Size            int64           `json:"size,omitempty"`
}

type InfoJobResponse struct {
	OK         bool          `json:"ok"`
	JobID      string        `json:"job_id,omitempty"`
	Status     string        `json:"status,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	Output     string        `json:"output,omitempty"`
	Error      string        `json:"error,omitempty"`
	Logs       string        `json:"logs,omitempty"`
	LogEntries []LogEntry    `json:"log_entries,omitempty"`
	Progress   *TaskProgress `json:"progress,omitempty"`
}

type ScreenshotFileInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type ScreenshotResponse struct {
	OK     bool                  `json:"ok"`
	Output string                `json:"output,omitempty"`
	Error  string                `json:"error,omitempty"`
	Logs   string                `json:"logs,omitempty"`
	Files  []ScreenshotFileInfo  `json:"files,omitempty"`
}

type PathResponse struct {
	OK    bool     `json:"ok"`
	Root  string   `json:"root,omitempty"`
	Roots []string `json:"roots,omitempty"`
	Items []any    `json:"items,omitempty"`
	Error string   `json:"error,omitempty"`
}
