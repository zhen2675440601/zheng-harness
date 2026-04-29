//go:build !windows

package plugin

import (
	"context"
	"errors"
	"fmt"
	stdplugin "plugin"
	"strings"
)

var ErrNativePluginSymbolNotFound = errors.New("native plugin symbol not found")

// NativeLoader loads Go plugins built with -buildmode=plugin on non-Windows platforms.
type NativeLoader struct {
	Path string
}

// CanLoad reports whether the provided path looks like a native Go plugin artifact.
func (l NativeLoader) CanLoad(path string) bool {
	if path == "" {
		path = l.Path
	}
	return strings.HasSuffix(strings.ToLower(path), ".so")
}

// Load opens the shared object, resolves NewPluginTool, and validates the plugin contract.
func (l NativeLoader) Load(ctx context.Context) (PluginTool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l.Path == "" {
		return nil, fmt.Errorf("native plugin path must not be empty")
	}
	if !l.CanLoad("") {
		return nil, fmt.Errorf("native plugin must use .so extension: %s", l.Path)
	}

	plug, err := stdplugin.Open(l.Path)
	if err != nil {
		return nil, fmt.Errorf("open native plugin %q: %w", l.Path, err)
	}

	symbol, err := plug.Lookup("NewPluginTool")
	if err != nil {
		return nil, fmt.Errorf("%w: NewPluginTool: %v", ErrNativePluginSymbolNotFound, err)
	}

	factory, err := nativePluginFactory(symbol)
	if err != nil {
		return nil, err
	}

	tool, err := factory()
	if err != nil {
		return nil, fmt.Errorf("construct native plugin tool: %w", err)
	}
	if err := ValidateContract(tool); err != nil {
		return nil, err
	}
	return tool, nil
}

func nativePluginFactory(symbol any) (func() (PluginTool, error), error) {
	switch factory := symbol.(type) {
	case func() PluginTool:
		return func() (PluginTool, error) {
			return factory(), nil
		}, nil
	case func() (PluginTool, error):
		return factory, nil
	default:
		return nil, fmt.Errorf("native plugin NewPluginTool has unsupported signature %T", symbol)
	}
}
