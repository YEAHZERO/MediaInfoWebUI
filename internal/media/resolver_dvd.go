package media

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func resolveDVDMediaInfoFileFromRoot(root string) (string, error) {
	dvdRoot, ok := resolveDVDVideoRoot(root)
	if !ok {
		return "", errors.New("VIDEO_TS folder not found")
	}

	titleVOB, err := findMainDVDTitleSetFirstVOB(dvdRoot)
	if err == nil {
		ifoPath := dvdControlIFOPathFromTitleVOB(titleVOB)
		if ifoPath != "" {
			if info, statErr := os.Stat(ifoPath); statErr == nil && !info.IsDir() {
				return ifoPath, nil
			}
		}
		return titleVOB, nil
	}

	videoTSIFO := filepath.Join(dvdRoot, "VIDEO_TS.IFO")
	if info, statErr := os.Stat(videoTSIFO); statErr == nil && !info.IsDir() {
		return videoTSIFO, nil
	}
	return "", err
}

func findMainDVDTitleSetFirstVOB(videoTSDir string) (string, error) {
	entries, err := os.ReadDir(videoTSDir)
	if err != nil {
		return "", err
	}

	type dvdTitleVOB struct {
		titleSet int
		part     int
		size     int64
		path     string
	}

	items := make([]dvdTitleVOB, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isDVDTitleVOBName(entry.Name()) {
			continue
		}
		titleSet, part, ok := parseDVDTitleVOBName(entry.Name())
		if !ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		items = append(items, dvdTitleVOB{
			titleSet: titleSet,
			part:     part,
			size:     info.Size(),
			path:     filepath.Join(videoTSDir, entry.Name()),
		})
	}
	if len(items) == 0 {
		return "", errors.New("no DVD title VOB files found under VIDEO_TS")
	}

	titleSetSizes := make(map[int]int64, len(items))
	for _, item := range items {
		titleSetSizes[item.titleSet] += item.size
	}

	sort.Slice(items, func(i, j int) bool {
		leftTotal := titleSetSizes[items[i].titleSet]
		rightTotal := titleSetSizes[items[j].titleSet]
		if leftTotal != rightTotal {
			return leftTotal > rightTotal
		}
		if items[i].titleSet != items[j].titleSet {
			return items[i].titleSet < items[j].titleSet
		}
		if items[i].part != items[j].part {
			return items[i].part < items[j].part
		}
		return items[i].path < items[j].path
	})

	mainTitleSet := items[0].titleSet
	best := items[0]
	for _, item := range items[1:] {
		if item.titleSet != mainTitleSet {
			break
		}
		if item.part < best.part || (item.part == best.part && item.path < best.path) {
			best = item
		}
	}
	return best.path, nil
}

func dvdControlIFOPathFromTitleVOB(path string) string {
	base := filepath.Base(path)
	if len(base) < len("VTS_00_1.VOB") {
		return ""
	}
	if !isDVDTitleVOBName(base) {
		return ""
	}
	return filepath.Join(filepath.Dir(path), base[:7]+"0.IFO")
}

func dvdTitleVOBPathFromControlFile(path string) string {
	base := strings.ToUpper(filepath.Base(path))
	if len(base) < len("VTS_00_0.IFO") {
		return ""
	}
	if !isDVDTitleControlFileName(base) {
		return ""
	}

	vobPath := filepath.Join(filepath.Dir(path), base[:7]+"1.VOB")
	info, err := os.Stat(vobPath)
	if err != nil || info.IsDir() {
		return ""
	}
	return vobPath
}

func isDVDTitleControlFileName(name string) bool {
	name = strings.ToUpper(strings.TrimSpace(name))
	return len(name) == len("VTS_00_0.IFO") &&
		strings.HasPrefix(name, "VTS_") &&
		name[6] == '_' &&
		name[7] == '0' &&
		name[8] == '.' &&
		name[4] >= '0' && name[4] <= '9' &&
		name[5] >= '0' && name[5] <= '9' &&
		(strings.HasSuffix(name, ".IFO") || strings.HasSuffix(name, ".BUP"))
}

func isDVDTitleVOBName(name string) bool {
	name = strings.ToUpper(strings.TrimSpace(name))
	return len(name) == len("VTS_00_1.VOB") &&
		strings.HasPrefix(name, "VTS_") &&
		name[6] == '_' &&
		name[8] == '.' &&
		name[7] >= '1' && name[7] <= '9' &&
		name[4] >= '0' && name[4] <= '9' &&
		name[5] >= '0' && name[5] <= '9' &&
		strings.HasSuffix(name, ".VOB")
}

func parseDVDTitleVOBName(name string) (int, int, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if !isDVDTitleVOBName(name) {
		return 0, 0, false
	}

	titleSet := int(name[4]-'0')*10 + int(name[5]-'0')
	part := int(name[7] - '0')
	return titleSet, part, true
}

func resolveDVDVideoRoot(path string) (string, bool) {
	base := filepath.Base(path)
	if strings.EqualFold(base, "VIDEO_TS") {
		return path, true
	}
	videoTs := filepath.Join(path, "VIDEO_TS")
	if info, err := os.Stat(videoTs); err == nil && info.IsDir() {
		return videoTs, true
	}
	return "", false
}
