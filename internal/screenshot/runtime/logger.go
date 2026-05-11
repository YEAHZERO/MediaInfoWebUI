package runtime

import (
	"fmt"
	"strings"
)

type Logger struct {
	lines   []string
	handler LineHandler
}

func NewLogger(handler LineHandler) Logger {
	return Logger{handler: handler}
}

func (l *Logger) Addf(format string, args ...interface{}) {
	if l == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	l.lines = append(l.lines, line)
	if l.handler != nil {
		l.handler(line)
	}
}

func (l *Logger) Text() string {
	if l == nil {
		return ""
	}
	return strings.TrimSpace(strings.Join(l.lines, "\n"))
}

const (
	ActiveShotPhaseRender   = "render"
	ActiveShotPhaseReencode = "reencode"
)

type ActiveShot struct {
	index int
	total int
	name  string
	phase string
}

func (s *ActiveShot) Prepare(index, total int) {
	if s == nil { return }
	s.index = index; s.total = total; s.name = ""; s.phase = ""
}

func (s *ActiveShot) BeginRender(index, total int, name string) {
	if s == nil { return }
	s.index = index; s.total = total
	s.name = strings.TrimSpace(name)
	s.phase = ActiveShotPhaseRender
}

func (s *ActiveShot) SetPhase(phase string) {
	if s == nil { return }
	s.phase = strings.TrimSpace(phase)
}

func (s *ActiveShot) Reset() {
	if s == nil { return }
	s.index = 0; s.total = 0; s.name = ""; s.phase = ""
}

func (s ActiveShot) Current() int { return s.index }
func (s ActiveShot) Total() int   { return s.total }
func (s ActiveShot) Phase() string { return s.phase }
func (s ActiveShot) Active() bool  { return s.index > 0 && s.total > 0 }

func (s ActiveShot) ProgressLabel() string {
	if !s.Active() || s.name == "" { return "正在渲染截图。" }
	phase := s.phase
	if phase == ActiveShotPhaseReencode { phase = "重拍" } else { phase = "渲染" }
	return fmt.Sprintf("正在%s第 %d/%d 张截图：%s", phase, s.index, s.total, s.name)
}

func (s ActiveShot) AlignmentDetail() string {
	if !s.Active() { return "" }
	return fmt.Sprintf("正在对齐第 %d/%d 张截图时间点...", s.index, s.total)
}

func (s ActiveShot) BitmapVisibilityDetail(label string) string {
	if !s.Active() { return "" }
	if label == "" { label = "PGS/DVD" }
	return fmt.Sprintf("正在校验 %s 字幕是否可见...", label)
}