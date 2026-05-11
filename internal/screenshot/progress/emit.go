package progress

import "fmt"

func EmitStepLog(onLog LineHandler, stage string, current, total int, detail string) {
	if onLog == nil {
		return
	}
	onLog(fmt.Sprintf("[进度] %s %d/%d: %s", stage, current, total, detail))
}

func EmitPercentLog(onLog LineHandler, stage string, percent float64, detail string) {
	if onLog == nil {
		return
	}
	onLog(fmt.Sprintf("[进度] %s %.1f%%: %s", stage, percent, detail))
}