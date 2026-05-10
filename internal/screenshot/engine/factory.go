package engine

import (
	"os"
	"strings"
)

func New() ScreenshotEngine {
	if isNativeEnabled() {
		if eng := tryNative(); eng != nil {
			return eng
		}
	}
	return newScriptEngine()
}

func isNativeEnabled() bool {
	return strings.TrimSpace(os.Getenv("ENABLE_NATIVE_ENGINE")) == "1"
}

var nativeConstructor func() ScreenshotEngine