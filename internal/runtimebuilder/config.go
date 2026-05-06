package runtimebuilder

import (
	"strings"

	"zheng-harness/internal/config"
)

var configFlagNames = map[string]bool{
	"config":          true,
	"model":           true,
	"provider":        true,
	"plugin-provider": true,
	"max-steps":       true,
	"step-timeout":    true,
	"memory-limit-mb": true,
	"verify-mode":     true,
	"api-key":         true,
	"base-url":        true,
	"listen-address":  true,
	"jwt-secret":      true,
	"jwt-secret-file": true,
	"active-session-cap": true,
	"shutdown-timeout": true,
	"server-enable-wal": true,
}

func LoadCLIConfig(command string, args []string) (config.Config, error) {
	cfg := config.Default()
	if command != "run" && command != "resume" {
		return cfg, nil
	}
	return config.Load(FilterConfigArgs(args))
}

func FilterConfigArgs(args []string) []string {
	var filtered []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		flagName := strings.TrimLeft(arg, "-")
		if equalsIndex := strings.Index(flagName, "="); equalsIndex != -1 {
			flagName = flagName[:equalsIndex]
		}
		if !configFlagNames[flagName] {
			continue
		}
		filtered = append(filtered, arg)
		if !strings.Contains(arg, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			filtered = append(filtered, args[i])
		}
	}
	return filtered
}
