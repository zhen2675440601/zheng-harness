package adapters

import (
	"bytes"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"zheng-harness/internal/domain"
)

const (
	defaultCodeSearchMaxResults = 50
	codeSearchContextLines      = 2
)

var (
	errCodeSearchMaxResultsReached = errors.New("code_search max results reached")
	codeSearchLanguageExtensions   = map[string][]string{
		"go":         {".go"},
		"python":     {".py"},
		"javascript": {".js", ".jsx", ".mjs", ".cjs"},
		"typescript": {".ts", ".tsx", ".mts", ".cts"},
		"java":       {".java"},
		"rust":       {".rs"},
	}
	defaultCodeExtensions = map[string]struct{}{
		".go": {}, ".py": {}, ".js": {}, ".jsx": {}, ".mjs": {}, ".cjs": {},
		".ts": {}, ".tsx": {}, ".mts": {}, ".cts": {}, ".java": {}, ".rs": {},
		".c": {}, ".cc": {}, ".cpp": {}, ".cxx": {}, ".h": {}, ".hh": {}, ".hpp": {}, ".hxx": {},
		".cs": {}, ".rb": {}, ".php": {}, ".swift": {}, ".kt": {}, ".kts": {}, ".scala": {},
		".sh": {}, ".bash": {}, ".zsh": {}, ".ps1": {}, ".sql": {},
		".html": {}, ".css": {}, ".scss": {}, ".sass": {}, ".less": {},
		".json": {}, ".yaml": {}, ".yml": {}, ".xml": {}, ".toml": {},
	}
	excludedCodeSearchDirs = map[string]struct{}{
		".git":         {},
		".sisyphus":    {},
		"vendor":       {},
		"node_modules": {},
	}
)

// CodeSearchAdapter executes language-aware regex search against source files.
type CodeSearchAdapter struct {
	workspaceRoot string
}

// NewCodeSearchAdapter constructs a code-scoped search adapter.
func NewCodeSearchAdapter(workspaceRoot string) CodeSearchAdapter {
	return CodeSearchAdapter{workspaceRoot: workspaceRoot}
}

func (a CodeSearchAdapter) Search(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	input, err := parseCodeSearchInput(call.Input)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	compiled, err := regexp.Compile(input.Pattern)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Error: err.Error(), Duration: time.Since(start)}, nil
	}

	root, err := filepath.Abs(a.workspaceRoot)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	collector, err := newCodeSearchCollector(input.OutputMode, input.MaxResults)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Error: err.Error(), Duration: time.Since(start)}, nil
	}

	allowedExtensions, err := resolveLanguageExtensions(input.Language)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Error: err.Error(), Duration: time.Since(start)}, nil
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			if shouldSkipCodeSearchDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if shouldSkipCodeSearchFile(rel, allowedExtensions) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if isBinaryContent(content) {
			return nil
		}

		if err := collector.collect(rel, string(content), compiled); err != nil {
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, errCodeSearchMaxResultsReached) {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	return domain.ToolResult{ToolName: call.Name, Output: collector.output(), Duration: time.Since(start)}, nil
}

type codeSearchInput struct {
	Pattern    string `json:"pattern"`
	Language   string `json:"language"`
	OutputMode string `json:"output_mode"`
	MaxResults int    `json:"max_results"`
}

func parseCodeSearchInput(raw string) (codeSearchInput, error) {
	var input codeSearchInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return codeSearchInput{}, fmt.Errorf("code_search input must be valid JSON: %w", err)
	}
	input.Pattern = strings.TrimSpace(input.Pattern)
	if input.Pattern == "" {
		return codeSearchInput{}, fmt.Errorf("code_search pattern must not be empty")
	}
	input.Language = strings.ToLower(strings.TrimSpace(input.Language))
	input.OutputMode = normalizeCodeSearchOutputMode(strings.TrimSpace(input.OutputMode))
	if input.MaxResults == 0 {
		input.MaxResults = defaultCodeSearchMaxResults
	}
	if input.MaxResults < 0 {
		return codeSearchInput{}, fmt.Errorf("code_search max_results must be non-negative")
	}
	return input, nil
}

func normalizeCodeSearchOutputMode(mode string) string {
	if mode == "" {
		return "files_with_matches"
	}
	return mode
}

func resolveLanguageExtensions(language string) (map[string]struct{}, error) {
	if language == "" {
		return defaultCodeExtensions, nil
	}
	extensions, ok := codeSearchLanguageExtensions[language]
	if !ok {
		return nil, fmt.Errorf("unsupported code_search language %q", language)
	}
	resolved := make(map[string]struct{}, len(extensions))
	for _, ext := range extensions {
		resolved[ext] = struct{}{}
	}
	return resolved, nil
}

func shouldSkipCodeSearchDir(name string) bool {
	_, skip := excludedCodeSearchDirs[name]
	return skip
}

func shouldSkipCodeSearchFile(rel string, allowedExtensions map[string]struct{}) bool {
	base := strings.ToLower(filepath.Base(rel))
	if strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".min.css") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(base))
	_, ok := allowedExtensions[ext]
	return !ok
}

func isBinaryContent(content []byte) bool {
	return bytes.IndexByte(content, 0) >= 0
}

type codeSearchCollector struct {
	mode          string
	maxResults    int
	files         map[string]struct{}
	counts        map[string]int
	contentBlocks []codeSearchContentBlock
	results       int
}

type codeSearchContentBlock struct {
	file  string
	start int
	end   int
	lines []string
}

func newCodeSearchCollector(mode string, maxResults int) (*codeSearchCollector, error) {
	switch mode {
	case "files_with_matches", "content", "count":
		return &codeSearchCollector{
			mode:       mode,
			maxResults: maxResults,
			files:      make(map[string]struct{}),
			counts:     make(map[string]int),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported code_search output mode %q", mode)
	}
}

func (c *codeSearchCollector) collect(rel string, content string, compiled *regexp.Regexp) error {
	lines := scanLines(content)
	matchLines := make([]int, 0)
	for i, line := range lines {
		if compiled.MatchString(line) {
			matchLines = append(matchLines, i+1)
		}
	}
	if len(matchLines) == 0 {
		return nil
	}

	switch c.mode {
	case "content":
		for _, window := range mergeContextWindows(matchLines, len(lines), codeSearchContextLines) {
			if c.maxResults > 0 && c.results >= c.maxResults {
				return errCodeSearchMaxResultsReached
			}
			block := codeSearchContentBlock{file: rel, start: window[0], end: window[1]}
			for lineNumber := window[0]; lineNumber <= window[1]; lineNumber++ {
				block.lines = append(block.lines, fmt.Sprintf("%s:%d: %s", rel, lineNumber, lines[lineNumber-1]))
			}
			c.contentBlocks = append(c.contentBlocks, block)
			c.results++
		}
	case "count":
		if c.maxResults > 0 && c.results >= c.maxResults {
			return errCodeSearchMaxResultsReached
		}
		c.counts[rel] = len(matchLines)
		c.results++
	default:
		if c.maxResults > 0 && c.results >= c.maxResults {
			return errCodeSearchMaxResultsReached
		}
		c.files[rel] = struct{}{}
		c.results++
	}
	return nil
}

func (c *codeSearchCollector) output() string {
	switch c.mode {
	case "content":
		parts := make([]string, 0, len(c.contentBlocks))
		for _, block := range c.contentBlocks {
			parts = append(parts, strings.Join(block.lines, "\n"))
		}
		return strings.Join(parts, "\n--\n")
	case "count":
		keys := sortedKeys(c.counts)
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s:%d", key, c.counts[key]))
		}
		return strings.Join(lines, "\n")
	default:
		return strings.Join(sortedSetKeys(c.files), "\n")
	}
}

func scanLines(content string) []string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func mergeContextWindows(matchLines []int, totalLines int, contextLines int) [][2]int {
	if len(matchLines) == 0 {
		return nil
	}
	windows := make([][2]int, 0, len(matchLines))
	for _, line := range matchLines {
		start := max(1, line-contextLines)
		end := min(totalLines, line+contextLines)
		if len(windows) == 0 || start > windows[len(windows)-1][1]+1 {
			windows = append(windows, [2]int{start, end})
			continue
		}
		if end > windows[len(windows)-1][1] {
			windows[len(windows)-1][1] = end
		}
	}
	return windows
}
