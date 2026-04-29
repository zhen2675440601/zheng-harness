package adapters

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v2"

	"zheng-harness/internal/domain"
)

// SearchAdapter 在工作区文件中执行简单的文本搜索。
type SearchAdapter struct {
	workspaceRoot string
}

// NewSearchAdapter 构造一个工作区范围内的搜索适配器。
func NewSearchAdapter(workspaceRoot string) SearchAdapter {
	return SearchAdapter{workspaceRoot: workspaceRoot}
}

func (a SearchAdapter) Grep(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	query, parseErr := parseSearchInput(call.Input)
	if parseErr != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, parseErr
	}
	if query.pattern == "" {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, fmt.Errorf("search term must not be empty")
	}

	compiled, err := compilePattern(query.pattern, query.flags)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Error: err.Error(), Duration: time.Since(start)}, nil
	}

	root, err := filepath.Abs(a.workspaceRoot)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	collector, err := newSearchCollector(query.outputMode)
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
			if d.Name() == ".git" || d.Name() == ".sisyphus" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if query.includeGlob != "" {
			matched, err := doublestar.Match(query.includeGlob, rel)
			if err != nil {
				return err
			}
			if !matched {
				return nil
			}
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		collector.collect(rel, string(content), compiled)
		return nil
	})
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	return domain.ToolResult{ToolName: call.Name, Output: collector.output(), Duration: time.Since(start)}, nil
}

type searchQuery struct {
	pattern    string
	flags      string
	outputMode string
	includeGlob string
}

func parseSearchInput(input string) (searchQuery, error) {
	parts := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	for len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return searchQuery{}, nil
	}
	if len(parts) > 4 {
		return searchQuery{}, fmt.Errorf("grep_search input supports up to four lines: pattern, flags, output mode, include glob")
	}
	query := searchQuery{pattern: strings.TrimSpace(parts[0]), outputMode: "files_with_matches"}
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		query.flags = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		query.outputMode = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" {
		query.includeGlob = strings.TrimSpace(parts[3])
	}
	return query, nil
}

func compilePattern(pattern string, flags string) (*regexp.Regexp, error) {
	if flags == "" {
		return regexp.Compile(pattern)
	}
	if !strings.ContainsAny(flags, "im") {
		return nil, fmt.Errorf("unsupported regex flags %q", flags)
	}
	for _, flag := range flags {
		if flag != 'i' && flag != 'm' {
			return nil, fmt.Errorf("unsupported regex flags %q", flags)
		}
	}
	return regexp.Compile("(?" + flags + ")" + pattern)
}

type searchCollector struct {
	mode          string
	files         map[string]struct{}
	contentMatches []string
	counts        map[string]int
}

func newSearchCollector(mode string) (*searchCollector, error) {
	switch mode {
	case "", "files_with_matches", "content", "count":
		return &searchCollector{
			mode:   normalizeOutputMode(mode),
			files:  make(map[string]struct{}),
			counts: make(map[string]int),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported grep_search output mode %q", mode)
	}
}

func normalizeOutputMode(mode string) string {
	if mode == "" {
		return "files_with_matches"
	}
	return mode
}

func (c *searchCollector) collect(rel string, content string, compiled *regexp.Regexp) {
	matchCount := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if !compiled.MatchString(line) {
			continue
		}
		matchCount++
		if c.mode == "content" {
			c.contentMatches = append(c.contentMatches, fmt.Sprintf("%s:%d: %s", rel, lineNumber, line))
		}
	}
	if matchCount == 0 {
		return
	}
	c.files[rel] = struct{}{}
	c.counts[rel] = matchCount
}

func (c *searchCollector) output() string {
	switch c.mode {
	case "content":
		return strings.Join(c.contentMatches, "\n")
	case "count":
		keys := sortedKeys(c.counts)
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("%s:%d", key, c.counts[key]))
		}
		return strings.Join(lines, "\n")
	default:
		keys := sortedSetKeys(c.files)
		return strings.Join(keys, "\n")
	}
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
