//go:build windows

package plugin

import (
	"context"
	"errors"
)

var (
	ErrNativePluginsUnsupported = errors.New("native plugins not supported on Windows")
	ErrNativePluginSymbolNotFound = errors.New("native plugin symbol not found")
)

// NativeLoader rejects native Go plugins on Windows.
type NativeLoader struct {
	Path string
}

// CanLoad always returns false on Windows.
func (l NativeLoader) CanLoad(path string) bool {
	return false
}

// Load always returns the Windows unsupported error.
func (l NativeLoader) Load(ctx context.Context) (PluginTool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrNativePluginsUnsupported
}
