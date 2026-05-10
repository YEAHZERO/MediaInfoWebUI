//go:build !native

package engine

func tryNative() ScreenshotEngine {
	return nil
}