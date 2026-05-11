package system

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"mediainfo/internal/config"
)

func ResolveBin(envKey, fallback string) (string, error) {
	bin := config.Getenv(envKey, fallback)
	if _, err := exec.LookPath(bin); err != nil {
		return "", fmt.Errorf("%s not found; set %s or add to PATH", bin, envKey)
	}
	return bin, nil
}

type OutputLineHandler func(stream, line string)

func RunCommand(ctx context.Context, bin string, args ...string) (string, string, error) {
	return runCommand(ctx, "", bin, args...)
}

func RunCommandInDir(ctx context.Context, dir, bin string, args ...string) (string, string, error) {
	return runCommand(ctx, dir, bin, args...)
}

func RunCommandLive(ctx context.Context, bin string, onLine OutputLineHandler, args ...string) (string, string, error) {
	return runCommandLive(ctx, "", bin, onLine, args...)
}

func RunCommandInDirLive(ctx context.Context, dir, bin string, onLine OutputLineHandler, args ...string) (string, string, error) {
	return runCommandLive(ctx, dir, bin, onLine, args...)
}

func runCommand(ctx context.Context, dir, bin string, args ...string) (string, string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = commandEnv(ctx, bin)
	setCommandProcessGroup(cmd)

	stdoutFile, err := os.CreateTemp("", "mediainfo-stdout-*")
	if err != nil {
		return "", "", err
	}
	defer os.Remove(stdoutFile.Name())
	defer stdoutFile.Close()

	stderrFile, err := os.CreateTemp("", "mediainfo-stderr-*")
	if err != nil {
		return "", "", err
	}
	defer os.Remove(stderrFile.Name())
	defer stderrFile.Close()

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		killCommandProcessGroup(cmd)
		waitErr = ctx.Err()
		<-waitCh
	}

	stdoutData, _ := os.ReadFile(stdoutFile.Name())
	stderrData, _ := os.ReadFile(stderrFile.Name())
	return string(stdoutData), string(stderrData), waitErr
}

func runCommandLive(ctx context.Context, dir, bin string, onLine OutputLineHandler, args ...string) (string, string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = commandEnv(ctx, bin)
	setCommandProcessGroup(cmd)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	stdoutWriter := io.Writer(&stdoutBuf)
	stderrWriter := io.Writer(&stderrBuf)

	var stdoutRelay *lineRelayWriter
	var stderrRelay *lineRelayWriter
	if onLine != nil {
		stdoutRelay = newLineRelayWriter("stdout", onLine)
		stderrRelay = newLineRelayWriter("stderr", onLine)
		stdoutWriter = io.MultiWriter(&stdoutBuf, stdoutRelay)
		stderrWriter = io.MultiWriter(&stderrBuf, stderrRelay)
	}

	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-waitCh:
	case <-ctx.Done():
		killCommandProcessGroup(cmd)
		waitErr = ctx.Err()
		<-waitCh
	}

	if stdoutRelay != nil {
		stdoutRelay.Flush()
	}
	if stderrRelay != nil {
		stderrRelay.Flush()
	}

	return stdoutBuf.String(), stderrBuf.String(), waitErr
}

type lineRelayWriter struct {
	stream string
	onLine OutputLineHandler
	buffer bytes.Buffer
}

func newLineRelayWriter(stream string, onLine OutputLineHandler) *lineRelayWriter {
	return &lineRelayWriter{stream: stream, onLine: onLine}
}

func (w *lineRelayWriter) Write(p []byte) (n int, err error) {
	w.buffer.Write(p)
	w.flushLines(false)
	return len(p), nil
}

func (w *lineRelayWriter) Flush() {
	w.flushLines(true)
}

func (w *lineRelayWriter) flushLines(flushAll bool) {
	for {
		line, err := w.buffer.ReadString('\n')
		if err != nil {
			if flushAll && w.buffer.Len() > 0 {
				remaining := w.buffer.String()
				w.onLine(w.stream, remaining)
			}
			break
		}
		w.onLine(w.stream, strings.TrimRight(line, "\n\r"))
	}
}

func BestErrorMessage(err error, stderr, stdout string) string {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = err.Error()
	}
	if strings.TrimSpace(stdout) != "" {
		msg += "\n\n" + strings.TrimSpace(stdout)
	}
	return msg
}

func CombineCommandOutput(stdout, stderr string) string {
	output := strings.TrimSpace(stdout)
	if strings.TrimSpace(stderr) != "" {
		if output != "" {
			output += "\n\n"
		}
		output += strings.TrimSpace(stderr)
	}
	return output
}
