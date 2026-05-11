package delivery

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ListImageFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	extSet := map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".webp": true,
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if !extSet[ext] {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}

	sort.Strings(files)
	return files, nil
}