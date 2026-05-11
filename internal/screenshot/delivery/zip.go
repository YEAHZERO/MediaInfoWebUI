package delivery

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
)

func ZipFiles(paths []string) ([]byte, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		name := filepath.Base(path)
		f, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(data); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}