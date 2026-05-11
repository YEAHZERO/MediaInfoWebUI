//go:build native

package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"mediainfo/internal/system"
)

type ffmpegRealtimeState struct {
	mu                sync.Mutex
	frame             string
	fps               string
	outTime           string
	outTimeMS         int64
	speed             string
	progressHeartbeat int
	startedAt         time.Time
	windowSeconds     float64
	firstOutTimeMS    int64
	hasFirstOutTime   bool
}

func consumeFFmpegProgressLine(line string, state *ffmpegRealtimeState, onProgress func(int, int, string)) {
	if line == "" {
		return
	}

	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return
	}

	state.mu.Lock()
	switch key {
	case "frame":
		state.frame = value
	case "fps":
		state.fps = value
	case "out_time":
		state.outTime = value
	case "out_time_ms":
		if parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
			state.outTimeMS = parsed
			if !state.hasFirstOutTime {
				state.firstOutTimeMS = parsed
				state.hasFirstOutTime = true
			}
		}
	case "speed":
		state.speed = value
	case "progress":
		emitRealtimeProgress(state, onProgress)
	}
	state.mu.Unlock()
}

func emitRealtimeProgress(state *ffmpegRealtimeState, onProgress func(int, int, string)) {
	if onProgress == nil {
		return
	}

	percent := approximatePercent(state)
	detail := progressDetail(state)
	onProgress(percent, 100, detail)
}

func approximatePercent(state *ffmpegRealtimeState) int {
	if state.windowSeconds <= 0 {
		state.progressHeartbeat++
		p := 10 + state.progressHeartbeat*10
		if p > 95 {
			p = 95
		}
		return p
	}

	if state.hasFirstOutTime && state.outTimeMS > state.firstOutTimeMS {
		processedSeconds := float64(state.outTimeMS-state.firstOutTimeMS) / 1_000_000.0
		if processedSeconds > 0 {
			p := int(processedSeconds / state.windowSeconds * 100)
			if p < 1 {
				p = 1
			}
			if p > 95 {
				p = 95
			}
			return p
		}
	}

	elapsed := time.Since(state.startedAt).Seconds()
	if elapsed <= 0 {
		return 5
	}

	estimate := state.windowSeconds
	if estimate < 1 {
		estimate = 1
	}
	p := int(95.0 * elapsed / (elapsed + estimate))
	if p < 1 {
		p = 1
	}
	return p
}

func progressDetail(state *ffmpegRealtimeState) string {
	parts := make([]string, 0, 3)
	if state.frame != "" {
		if f, err := strconv.Atoi(strings.TrimSpace(state.frame)); err == nil && f > 0 {
			parts = append(parts, "frame="+strings.TrimSpace(state.frame))
		}
	}
	if state.fps != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(state.fps), 64); err == nil && f > 0 {
			parts = append(parts, "fps="+strings.TrimSpace(state.fps))
		}
	}
	if state.speed != "" {
		speed := strings.TrimSpace(state.speed)
		if strings.HasSuffix(speed, "x") {
			parts = append(parts, "speed="+speed)
		}
	}
	if len(parts) == 0 {
		return "rendering..."
	}
	return strings.Join(parts, " | ")
}

func runFFmpegWithProgress(ctx *runContext, args []string, windowSeconds float64, onProgress func(int, int, string)) (string, string, error) {
	progressArgs := make([]string, 0, len(args)+4)
	progressArgs = append(progressArgs, "-progress", "pipe:1", "-nostats")

	if ctx.usesLibplacebo {
		progressArgs = append(progressArgs,
			"-init_hw_device", "vulkan=minfo:llvmpipe",
			"-filter_hw_device", "minfo",
		)
	}

	progressArgs = append(progressArgs, args...)

	state := &ffmpegRealtimeState{
		startedAt:     time.Now(),
		windowSeconds: windowSeconds,
	}

	done := make(chan struct{})
	defer close(done)

	if onProgress != nil {
		go func() {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					state.mu.Lock()
					emitRealtimeProgress(state, onProgress)
					state.mu.Unlock()
				}
			}
		}()
	}

	stdout, stderr, err := system.RunCommandLive(ctx.ctx, ctx.ffmpegBin, func(stream, line string) {
		if stream != "stdout" {
			return
		}
		consumeFFmpegProgressLine(strings.TrimSpace(line), state, onProgress)
	}, progressArgs...)
	if err != nil {
		message := strings.TrimSpace(stderr)
		if message == "" {
			message = err.Error()
		}
		return stdout, stderr, fmt.Errorf("%s", message)
	}
	return stdout, stderr, nil
}

type runContext struct {
	ctx          context.Context
	ffmpegBin    string
	usesLibplacebo bool
}

func isLibplaceboCrash(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "llvm error:") && strings.Contains(msg, "cannot select:") {
		return true
	}
	if strings.Contains(msg, "assertion failed:") && strings.Contains(msg, "pl_alloc.c") {
		return true
	}
	if strings.Contains(msg, "segmentation fault") {
		return true
	}
	return false
}

func runWithLibplaceboFallback(render func() error) error {
	if render == nil {
		return nil
	}

	err := render()
	if err == nil {
		return nil
	}
	if !isLibplaceboCrash(err) {
		return err
	}

	return render()
}