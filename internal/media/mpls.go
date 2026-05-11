package media

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const mplsClockRate = 45000

func isMPLSFile(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".mpls")
}

func resolveBDInfoPlaylistSelection(path string) (string, string, bool) {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" || !isMPLSFile(cleaned) {
		return "", "", false
	}

	playlistDir := filepath.Dir(cleaned)
	if !strings.EqualFold(filepath.Base(playlistDir), "PLAYLIST") {
		return "", "", false
	}

	bdmvDir := filepath.Dir(playlistDir)
	if !strings.EqualFold(filepath.Base(bdmvDir), "BDMV") {
		return "", "", false
	}

	root := filepath.Clean(filepath.Dir(bdmvDir))
	playlist := strings.ToUpper(filepath.Base(cleaned))
	if root == "" || playlist == "" {
		return "", "", false
	}
	return root, playlist, true
}

func readMPLSDuration(path string) (time.Duration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) < 18 {
		return 0, fmt.Errorf("mpls: file too short")
	}

	fileType := string(data[:8])
	switch fileType {
	case "MPLS0100", "MPLS0200", "MPLS0300":
	default:
		return 0, fmt.Errorf("mpls: unsupported file type %q", fileType)
	}

	playlistOffset := int(readUint32BE(data[8:12]))
	if playlistOffset < 0 || playlistOffset+10 > len(data) {
		return 0, fmt.Errorf("mpls: invalid playlist offset")
	}

	pos := playlistOffset
	pos += 4
	pos += 2
	itemCount := int(readUint16BE(data[pos : pos+2]))
	pos += 2
	pos += 2

	var totalTicks uint64
	for itemIndex := 0; itemIndex < itemCount; itemIndex++ {
		itemStart := pos
		if itemStart+2 > len(data) {
			return 0, fmt.Errorf("mpls: truncated playlist item header")
		}

		itemLength := int(readUint16BE(data[pos : pos+2]))
		itemEnd := itemStart + itemLength + 2
		if itemEnd > len(data) {
			return 0, fmt.Errorf("mpls: truncated playlist item body")
		}

		pos += 2
		pos += 5
		pos += 4
		pos += 1
		pos += 2
		if pos+8 > itemEnd {
			return 0, fmt.Errorf("mpls: truncated in/out time fields")
		}

		inTime := readUint32BE(data[pos : pos+4])
		pos += 4
		outTime := readUint32BE(data[pos : pos+4])
		pos += 4

		if outTime > inTime {
			totalTicks += uint64(outTime - inTime)
		}

		pos = itemEnd
	}

	if totalTicks == 0 {
		return 0, nil
	}

	return time.Duration(totalTicks * uint64(time.Second) / mplsClockRate), nil
}

func formatMPLSDuration(duration time.Duration) string {
	if duration <= 0 {
		return ""
	}

	totalSeconds := int64((duration + time.Second/2) / time.Second)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

func readUint16BE(data []byte) uint16 {
	return uint16(data[0])<<8 | uint16(data[1])
}

func readUint32BE(data []byte) uint32 {
	return uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
}