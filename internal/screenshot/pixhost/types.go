package pixhost

import screenshotruntime "mediainfo/internal/screenshot/runtime"

type UploadedImage struct {
	URL      string
	Filename string
	Size     int64
}

type UploadItemHandler func(item UploadedImage)

type LogHandler = screenshotruntime.LineHandler

type Result struct {
	Output       string
	Logs         string
	Items        []UploadedImage
	LossyIndexes []int
}