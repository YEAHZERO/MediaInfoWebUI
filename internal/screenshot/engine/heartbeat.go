package engine

import (
	"fmt"
	"time"
)

type heartbeat struct {
	callback ProgressCallback
	ticker   *time.Ticker
	done     chan struct{}
	phase    ProgressPhase
	current  int
	total    int
	start    time.Time
}

func startHeartbeat(cb ProgressCallback) *heartbeat {
	if cb == nil {
		return nil
	}
	h := &heartbeat{
		callback: cb,
		ticker:   time.NewTicker(500 * time.Millisecond),
		done:     make(chan struct{}),
		start:    time.Now(),
	}
	go h.run()
	return h
}

func (h *heartbeat) run() {
	for {
		select {
		case <-h.ticker.C:
			h.emit("")
		case <-h.done:
			h.ticker.Stop()
			return
		}
	}
}

func (h *heartbeat) update(phase ProgressPhase, current, total int, msg string) {
	if h == nil {
		return
	}
	h.phase = phase
	h.current = current
	h.total = total
	h.emit(msg)
}

func (h *heartbeat) emit(msg string) {
	if h == nil {
		return
	}
	h.callback(ProgressEvent{
		Phase:   h.phase,
		Current: h.current,
		Total:   h.total,
		Message: msg,
	})
}

func (h *heartbeat) doneMsg(msg string) {
	if h == nil {
		return
	}
	h.update(PhaseDone, h.total, h.total, msg)
}

func (h *heartbeat) stop() {
	if h == nil {
		return
	}
	select {
	case h.done <- struct{}{}:
	default:
	}
}

func (h *heartbeat) elapsed() string {
	if h == nil {
		return ""
	}
	d := time.Since(h.start)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}