package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"zheng-harness/internal/domain"
)

const defaultWebFetchMaxLength = 10000

// WebAdapter 执行受约束的 HTTP GET 请求。
type WebAdapter struct {
	allowedDomains []string
}

// NewWebAdapter 构造一个带域名允许列表的 Web 适配器。
func NewWebAdapter(allowedDomains []string) WebAdapter {
	copyOfDomains := append([]string(nil), allowedDomains...)
	return WebAdapter{allowedDomains: copyOfDomains}
}

func (a WebAdapter) Fetch(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	input, err := ParseWebFetchInput(call.Input)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	requestURL, err := ValidateWebFetchURL(input.URL)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	if err := validateAllowedDomain(requestURL, a.allowedDomains); err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	client := &http.Client{Timeout: remainingTimeout(ctx)}
	response, err := client.Do(request)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return domain.ToolResult{ToolName: call.Name, Duration: time.Since(start)}, err
	}

	return domain.ToolResult{
		ToolName: call.Name,
		Output:   truncateToMaxLength(string(body), input.MaxLength),
		Duration: time.Since(start),
	}, nil
}

type webFetchInput struct {
	URL       string `json:"url"`
	MaxLength int    `json:"max_length"`
}

// ParseWebFetchInput 解析 web_fetch JSON 输入。
func ParseWebFetchInput(raw string) (webFetchInput, error) {
	var input webFetchInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return webFetchInput{}, fmt.Errorf("web_fetch input must be valid JSON: %w", err)
	}
	input.URL = strings.TrimSpace(input.URL)
	if input.URL == "" {
		return webFetchInput{}, fmt.Errorf("web_fetch url must not be empty")
	}
	if input.MaxLength == 0 {
		input.MaxLength = defaultWebFetchMaxLength
	}
	if input.MaxLength < 0 {
		return webFetchInput{}, fmt.Errorf("web_fetch max_length must be non-negative")
	}
	return input, nil
}

// ValidateWebFetchURL 校验 web_fetch URL 仅允许 HTTP/S。
func ValidateWebFetchURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid web_fetch url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("web_fetch only supports http and https URLs")
	}
	if parsed.Hostname() == "" {
		return nil, fmt.Errorf("web_fetch url must include a host")
	}
	return parsed, nil
}

func validateAllowedDomain(requestURL *url.URL, allowedDomains []string) error {
	if len(allowedDomains) == 0 {
		return nil
	}
	hostname := strings.ToLower(requestURL.Hostname())
	for _, domain := range allowedDomains {
		if strings.EqualFold(strings.TrimSpace(domain), hostname) {
			return nil
		}
	}
	return fmt.Errorf("web_fetch domain %q is not allowed", requestURL.Hostname())
}

func remainingTimeout(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Nanosecond
	}
	return remaining
}

func truncateToMaxLength(body string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	if utf8.RuneCountInString(body) <= maxLength {
		return body
	}
	runes := []rune(body)
	return string(runes[:maxLength])
}
