package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"zheng-harness/internal/domain"
)

const defaultExternalPluginStartupTimeout = 5 * time.Second

var (
	ErrExternalPluginClosed   = errors.New("external plugin closed")
	ErrExternalPluginProtocol = errors.New("external plugin protocol error")
)

type externalInitializeParams struct {
	ContractVersion string `json:"contract_version"`
}

type externalInfoResult struct {
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Schema          string             `json:"schema"`
	Capabilities    []string           `json:"capabilities"`
	SafetyLevel     domain.SafetyLevel `json:"safety_level"`
	ContractVersion string             `json:"contract_version"`
}

type externalExecuteParams struct {
	Name      string `json:"name"`
	Input     string `json:"input"`
	TimeoutMS int64  `json:"timeout_ms,omitempty"`
}

type externalExecuteResult struct {
	ToolName string `json:"tool_name"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}

// ExternalLoader 启动外部进程插件并完成初始握手。
type ExternalLoader struct {
	Command        string
	Args           []string
	Dir            string
	Env            []string
	StartupTimeout time.Duration
}

// Load 启动插件进程，执行 initialize/tool.info 握手，并返回可执行工具。
func (l ExternalLoader) Load(ctx context.Context) (*ExternalPluginTool, error) {
	if l.Command == "" {
		return nil, fmt.Errorf("external plugin command must not be empty")
	}

	cmd := exec.CommandContext(context.Background(), l.Command, l.Args...)
	if l.Dir != "" {
		cmd.Dir = l.Dir
	}
	if len(l.Env) > 0 {
		cmd.Env = l.Env
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open plugin stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open plugin stdout: %w", err)
	}

	tool := &ExternalPluginTool{
		cmd:     cmd,
		stdin:   stdin,
		decoder: json.NewDecoder(stdout),
	}
	cmd.Stderr = &tool.stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start external plugin: %w", err)
	}

	tool.waitCh = make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		tool.exitMu.Lock()
		tool.exited = true
		tool.waitErr = waitErr
		tool.exitMu.Unlock()
		tool.waitCh <- waitErr
		close(tool.waitCh)
	}()

	startupTimeout := l.StartupTimeout
	if startupTimeout <= 0 {
		startupTimeout = defaultExternalPluginStartupTimeout
	}

	if err := tool.initialize(ctx, startupTimeout); err != nil {
		_ = tool.Close()
		return nil, err
	}

	if err := ValidateContract(tool); err != nil {
		_ = tool.Close()
		return nil, err
	}

	return tool, nil
}

// ExternalPluginTool 通过 JSON-RPC 2.0 over stdio 适配外部工具插件。
type ExternalPluginTool struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	decoder *json.Decoder
	stderr  bytes.Buffer

	rpcMu sync.Mutex
	id    uint64

	stateMu sync.RWMutex
	closed  bool

	exitMu  sync.RWMutex
	exited  bool
	waitErr error
	waitCh  chan error

	info externalInfoResult
}

func (t *ExternalPluginTool) Name() string {
	return t.info.Name
}

func (t *ExternalPluginTool) Description() string {
	return t.info.Description
}

func (t *ExternalPluginTool) Schema() string {
	return t.info.Schema
}

func (t *ExternalPluginTool) Capabilities() []string {
	return append([]string(nil), t.info.Capabilities...)
}

func (t *ExternalPluginTool) SafetyLevel() domain.SafetyLevel {
	return t.info.SafetyLevel
}

func (t *ExternalPluginTool) ContractVersion() string {
	return t.info.ContractVersion
}

func (t *ExternalPluginTool) Execute(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error) {
	start := time.Now()
	params := externalExecuteParams{
		Name:  call.Name,
		Input: call.Input,
	}
	if call.Timeout > 0 {
		params.TimeoutMS = call.Timeout.Milliseconds()
	}

	var payload externalExecuteResult
	if err := t.call(ctx, methodToolExecute, params, &payload); err != nil {
		return domain.ToolResult{ToolName: t.Name(), Duration: time.Since(start)}, err
	}

	toolName := payload.ToolName
	if toolName == "" {
		toolName = t.Name()
	}

	return domain.ToolResult{
		ToolName: toolName,
		Output:   payload.Output,
		Error:    payload.Error,
		Duration: nonZeroDuration(time.Since(start)),
	}, nil
}

func (t *ExternalPluginTool) Close() error {
	t.stateMu.Lock()
	if t.closed {
		t.stateMu.Unlock()
		return nil
	}
	t.closed = true
	t.stateMu.Unlock()

	if !t.hasExited() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultExternalPluginStartupTimeout)
		var acknowledged bool
		shutdownErr := t.callInternal(shutdownCtx, methodShutdown, nil, &acknowledged, true)
		cancel()
		if shutdownErr != nil && !errors.Is(shutdownErr, ErrExternalPluginClosed) {
			_ = t.killProcess()
			_ = t.wait()
			return shutdownErr
		}
	}

	if err := t.closeStdin(); err != nil {
		_ = t.killProcess()
		_ = t.wait()
		return err
	}

	return t.wait()
}

func (t *ExternalPluginTool) initialize(ctx context.Context, startupTimeout time.Duration) error {
	initCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	var initReply map[string]any
	if err := t.call(initCtx, methodInitialize, externalInitializeParams{ContractVersion: ContractVersion}, &initReply); err != nil {
		return fmt.Errorf("initialize external plugin: %w", err)
	}

	var info externalInfoResult
	if err := t.call(initCtx, methodToolInfo, nil, &info); err != nil {
		return fmt.Errorf("read external plugin info: %w", err)
	}
	if info.Name == "" {
		return fmt.Errorf("%w: plugin name must not be empty", ErrExternalPluginProtocol)
	}
	if info.ContractVersion == "" {
		return fmt.Errorf("%w: plugin contract version must not be empty", ErrExternalPluginProtocol)
	}

	t.info = info
	return nil
}

func (t *ExternalPluginTool) call(ctx context.Context, method string, params any, target any) error {
	return t.callInternal(ctx, method, params, target, false)
}

func (t *ExternalPluginTool) callInternal(ctx context.Context, method string, params any, target any, ignoreClosed bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	t.stateMu.RLock()
	closed := t.closed
	t.stateMu.RUnlock()
	if closed && !ignoreClosed {
		return ErrExternalPluginClosed
	}

	t.rpcMu.Lock()
	defer t.rpcMu.Unlock()

	t.id++
	requestID := t.id
	var rawParams json.RawMessage
	if params != nil {
		encodedParams, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal %s params: %w", method, err)
		}
		rawParams = encodedParams
	}
	request := JSONRPCRequest{
		JSONRPC: jsonRPCVersion,
		ID:      requestID,
		Method:  method,
		Params:  rawParams,
	}

	if err := json.NewEncoder(t.stdin).Encode(request); err != nil {
		t.markBroken()
		return t.wrapProcessError(fmt.Errorf("send %s request: %w", method, err))
	}

	type responseResult struct {
		response JSONRPCResponse
		err      error
	}
	responseCh := make(chan responseResult, 1)
	go func() {
		var response JSONRPCResponse
		decodeErr := t.decoder.Decode(&response)
		responseCh <- responseResult{response: response, err: decodeErr}
	}()

	select {
	case <-ctx.Done():
		t.markBroken()
		_ = t.killProcess()
		_ = t.wait()
		return fmt.Errorf("external plugin %s timeout: %w", method, ctx.Err())
	case result := <-responseCh:
		if result.err != nil {
			t.markBroken()
			_ = t.killProcess()
			_ = t.wait()
			return t.wrapProcessError(fmt.Errorf("decode %s response: %w", method, result.err))
		}
		if result.response.JSONRPC != jsonRPCVersion {
			return t.failProtocol("invalid jsonrpc version %q", result.response.JSONRPC)
		}
		if result.response.ID != requestID {
			return t.failProtocol("response id mismatch: got=%d want=%d", result.response.ID, requestID)
		}
		if result.response.Error != nil {
			return t.wrapProcessError(fmt.Errorf("remote %s error (%d): %s", method, result.response.Error.Code, result.response.Error.Message))
		}
		if target == nil || len(result.response.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(result.response.Result, target); err != nil {
			return t.failProtocol("decode %s result: %v", method, err)
		}
		return nil
	}
}

func (t *ExternalPluginTool) failProtocol(format string, args ...any) error {
	t.markBroken()
	_ = t.killProcess()
	_ = t.wait()
	return t.wrapProcessError(fmt.Errorf("%w: %s", ErrExternalPluginProtocol, fmt.Sprintf(format, args...)))
}

func (t *ExternalPluginTool) wrapProcessError(err error) error {
	stderr := t.stderr.String()
	if stderr == "" {
		return err
	}
	return fmt.Errorf("%w: stderr=%q", err, stderr)
}

func (t *ExternalPluginTool) markBroken() {
	t.stateMu.Lock()
	t.closed = true
	t.stateMu.Unlock()
}

func (t *ExternalPluginTool) hasExited() bool {
	t.exitMu.RLock()
	defer t.exitMu.RUnlock()
	return t.exited
}

func (t *ExternalPluginTool) closeStdin() error {
	if t.stdin == nil {
		return nil
	}
	err := t.stdin.Close()
	t.stdin = nil
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}

func (t *ExternalPluginTool) killProcess() error {
	if t.cmd == nil || t.cmd.Process == nil || t.hasExited() {
		return nil
	}
	if err := t.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func (t *ExternalPluginTool) wait() error {
	if t.waitCh == nil {
		return nil
	}
	if !t.hasExited() {
		for range t.waitCh {
		}
	}

	t.exitMu.RLock()
	defer t.exitMu.RUnlock()
	if t.waitErr == nil {
		return nil
	}
	if _, ok := t.waitErr.(*exec.ExitError); ok {
		return nil
	}
	return t.waitErr
}

func nonZeroDuration(value time.Duration) time.Duration {
	if value <= 0 {
		return time.Nanosecond
	}
	return value
}
