package screenshot

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
)

type ScreenshotFileInfo struct {
	Path string
	Name string
	Size int64
}

func ZipFiles(files []ScreenshotFileInfo) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, sf := range files {
		file, err := os.Open(sf.Path)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			_ = zw.Close()
			return nil, err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			file.Close()
			_ = zw.Close()
			return nil, err
		}
		header.Name = sf.Name
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			file.Close()
			_ = zw.Close()
			return nil, err
		}
		if _, err := io.Copy(writer, file); err != nil {
			file.Close()
			_ = zw.Close()
			return nil, err
		}
		file.Close()
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
