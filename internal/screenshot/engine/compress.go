package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mediainfo/internal/system"
)

const (
	defaultCompressThreshold = int64(10 * 1024 * 1024)

	StrategyAuto     = "auto"
	StrategyLossless = "lossless"
	StrategyLossy    = "lossy"
)

func CompressIfNeeded(ctx context.Context, path string, threshold int64, strategy string) (*CompressionResult, error) {
	if threshold <= 0 {
		threshold = defaultCompressThreshold
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	result := &CompressionResult{
		OriginalSize: info.Size(),
		Compressed:   false,
	}

	if info.Size() <= threshold {
		return result, nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return result, nil
	}

	if ext == ".png" {
		if err := doOxiPNG(ctx, path); err != nil {
			return result, fmt.Errorf("oxipng compress failed: %w", err)
		}

		afterInfo, statErr := os.Stat(path)
		if statErr == nil && afterInfo.Size() <= threshold {
			result.Compressed = true
			result.CompressedSize = afterInfo.Size()
			result.Method = "oxipng"
			result.Lossy = false
			return result, nil
		}

		if strategy == StrategyLossy || strategy == StrategyAuto {
			if err := doPNGQuant(ctx, path); err == nil {
				finalInfo, statErr := os.Stat(path)
				if statErr == nil {
					result.Compressed = true
					result.CompressedSize = finalInfo.Size()
					result.Method = "oxipng+pngquant"
					result.Lossy = true
					return result, nil
				}
			}
		}

		if statErr == nil {
			result.Compressed = true
			result.CompressedSize = afterInfo.Size()
			result.Method = "oxipng"
			result.Lossy = false
			return result, nil
		}
	}

	return result, nil
}

func doOxiPNG(ctx context.Context, path string) error {
	oxipng, err := system.ResolveBin("OXIPNG_BIN", "oxipng")
	if err != nil {
		return err
	}

	_, stderr, err := system.RunCommand(ctx, oxipng,
		"-o", "max",
		"--strip", "safe",
		"--quiet",
		path,
	)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf(msg)
	}
	return nil
}

func doPNGQuant(ctx context.Context, path string) error {
	pngquant, err := system.ResolveBin("PNGQUANT_BIN", "pngquant")
	if err != nil {
		return err
	}

	compressedPath := path + ".quant.png"
	_ = os.Remove(compressedPath)

	_, stderr, err := system.RunCommand(ctx, pngquant,
		"256",
		"--force",
		"--output", compressedPath,
		"--speed", "1",
		"--strip",
		"--",
		path,
	)
	if err != nil {
		_ = os.Remove(compressedPath)
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf(msg)
	}

	if err := os.Rename(compressedPath, path); err != nil {
		_ = os.Remove(compressedPath)
		return err
	}
	return nil
}