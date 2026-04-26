package main

import (
	"testing"
)

func TestFilterConfigArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: []string{},
		},
		{
			name:     "filters out --task",
			args:     []string{"--task", "test task", "--db", "./agent.db"},
			expected: []string{},
		},
		{
			name:     "keeps config flags",
			args:     []string{"--provider", "dashscope", "--model", "qwen3"},
			expected: []string{"--provider", "dashscope", "--model", "qwen3"},
		},
		{
			name:     "filters mixed args - config first",
			args:     []string{"--provider", "dashscope", "--task", "test", "--db", "./agent.db"},
			expected: []string{"--provider", "dashscope"},
		},
		{
			name:     "filters mixed args - config last",
			args:     []string{"--task", "test", "--provider", "dashscope"},
			expected: []string{"--provider", "dashscope"},
		},
		{
			name:     "handles equals syntax",
			args:     []string{"--model=qwen3", "--task=test"},
			expected: []string{"--model=qwen3"},
		},
		{
			name:     "keeps max-steps (config flag)",
			args:     []string{"--max-steps", "5", "--task", "test"},
			expected: []string{"--max-steps", "5"},
		},
		{
			name:     "keeps all config flags",
			args:     []string{"--config", "./config.json", "--provider", "dashscope", "--api-key", "sk-xxx", "--base-url", "https://api.example.com", "--max-steps", "10", "--step-timeout", "30s", "--memory-limit-mb", "256", "--verify-mode", "standard", "--model", "qwen3", "--allow-command", "npm"},
			expected: []string{"--config", "./config.json", "--provider", "dashscope", "--api-key", "sk-xxx", "--base-url", "https://api.example.com", "--max-steps", "10", "--step-timeout", "30s", "--memory-limit-mb", "256", "--verify-mode", "standard", "--model", "qwen3"},
		},
		{
			name:     "filters out --session and --db",
			args:     []string{"--session", "abc123", "--db", "./agent.db", "--json"},
			expected: []string{},
		},
		{
			name:     "single dash flags",
			args:     []string{"-task", "test", "-provider", "openai"},
			expected: []string{"-provider", "openai"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterConfigArgs(tt.args)
			if len(result) != len(tt.expected) {
				t.Errorf("filterConfigArgs(%v) = %v, want %v", tt.args, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("filterConfigArgs(%v)[%d] = %v, want %v", tt.args, i, result[i], tt.expected[i])
				}
			}
		})
	}
}
