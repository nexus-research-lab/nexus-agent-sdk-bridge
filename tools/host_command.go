package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// HostCommandOptions 描述 bridge 侧 MCP host tool 的 shell 适配入口。
type HostCommandOptions struct {
	Name             string
	Description      string
	Command          string
	InputSchema      map[string]any
	Env              map[string]string
	WorkingDirectory string
	Timeout          time.Duration
}

// NewHostCommandTool 创建一个 bridge-owned MCP tool，用 JSON stdin 调用宿主命令。
func NewHostCommandTool(options HostCommandOptions, toolOptions ...ToolOption) (Definition, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		return Definition{}, fmt.Errorf("tools: host command tool name is required")
	}
	command := strings.TrimSpace(options.Command)
	if command == "" {
		return Definition{}, fmt.Errorf("tools: host command for %q is required", name)
	}
	schema := options.InputSchema
	if schema == nil {
		schema = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	timeout := options.Timeout
	return New(name, options.Description, schema, func(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
		return runHostCommandTool(ctx, hostCommandRunOptions{
			Command:          command,
			Env:              options.Env,
			Input:            input,
			Timeout:          timeout,
			WorkingDirectory: options.WorkingDirectory,
		})
	}, toolOptions...), nil
}

type hostCommandRunOptions struct {
	Command          string
	Env              map[string]string
	Input            map[string]any
	Timeout          time.Duration
	WorkingDirectory string
}

func runHostCommandTool(ctx context.Context, options hostCommandRunOptions) (Result, error) {
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}
	payload, err := json.Marshal(options.Input)
	if err != nil {
		return Result{}, err
	}
	cmd := hostCommand(ctx, options.Command)
	if strings.TrimSpace(options.WorkingDirectory) != "" {
		cmd.Dir = options.WorkingDirectory
	}
	cmd.Env = os.Environ()
	for key, value := range options.Env {
		if strings.TrimSpace(key) != "" {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return Error(message), nil
	}
	return parseHostCommandResult(stdout.Bytes()), nil
}

func hostCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func parseHostCommandResult(output []byte) Result {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return Text("")
	}
	var mcpResult struct {
		Content           []map[string]any `json:"content"`
		StructuredContent map[string]any   `json:"structuredContent"`
		Meta              map[string]any   `json:"_meta"`
		IsError           bool             `json:"isError"`
	}
	if err := json.Unmarshal(trimmed, &mcpResult); err == nil && (len(mcpResult.Content) > 0 || len(mcpResult.StructuredContent) > 0 || len(mcpResult.Meta) > 0 || mcpResult.IsError) {
		return Result{
			Content:           mcpResult.Content,
			StructuredContent: mcpResult.StructuredContent,
			Meta:              mcpResult.Meta,
			IsError:           mcpResult.IsError,
		}
	}
	var object map[string]any
	if err := json.Unmarshal(trimmed, &object); err == nil {
		return Result{
			Content:           []map[string]any{{"type": "text", "text": string(trimmed)}},
			StructuredContent: object,
		}
	}
	return Text(string(trimmed))
}
