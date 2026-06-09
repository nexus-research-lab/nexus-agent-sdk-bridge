package nxs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const commandPathEnvName = "NEXUS_NXS_COMMAND_PATH"

// RuntimeSource 表示 nxs runtime 路径来源。
type RuntimeSource string

const (
	RuntimeSourceEnv RuntimeSource = "env"
)

// StatusError 表示 nxs runtime 探测失败的机器可读原因。
type StatusError string

const (
	StatusErrorNotFound         StatusError = "not_found"
	StatusErrorEnvNotExecutable StatusError = "env_not_executable"
)

// Status 表示 nxs runtime 在当前主机上的本地可用状态。
type Status struct {
	Available   bool          `json:"available"`
	Path        string        `json:"path,omitempty"`
	Source      RuntimeSource `json:"source,omitempty"`
	CanDownload bool          `json:"can_download"`
	Error       StatusError   `json:"error,omitempty"`
}

// RuntimeInspector 负责探测明确配置的 nxs runtime。
type RuntimeInspector struct {
	goos   string
	getenv func(string) string
	stat   func(string) (os.FileInfo, error)
}

// RuntimeInspectorOption 配置 RuntimeInspector。
type RuntimeInspectorOption func(*RuntimeInspector)

// NewRuntimeInspector 创建 nxs runtime 探测器。
func NewRuntimeInspector(options ...RuntimeInspectorOption) *RuntimeInspector {
	inspector := &RuntimeInspector{
		goos:   runtime.GOOS,
		getenv: os.Getenv,
		stat:   os.Stat,
	}
	for _, option := range options {
		if option != nil {
			option(inspector)
		}
	}
	return inspector.withDefaults()
}

// WithPlatform 设置探测平台，主要用于测试。
func WithPlatform(goos string, _ string) RuntimeInspectorOption {
	return func(inspector *RuntimeInspector) {
		inspector.goos = strings.TrimSpace(goos)
	}
}

// InspectRuntime 返回当前平台 nxs runtime 本地状态。
func InspectRuntime(options ...RuntimeInspectorOption) Status {
	return NewRuntimeInspector(options...).Status()
}

// EnsureRuntime 检查当前平台 nxs runtime 是否已明确配置。
func EnsureRuntime(options ...RuntimeInspectorOption) (Status, error) {
	return NewRuntimeInspector(options...).Ensure()
}

// Status 返回 nxs runtime 本地状态，不触发下载或路径猜测。
func (i *RuntimeInspector) Status() Status {
	inspector := i.withDefaults()
	if status, ok := inspector.statusFromEnv(); ok {
		return status
	}
	return Status{
		Available:   false,
		CanDownload: false,
		Error:       StatusErrorNotFound,
	}
}

// Ensure 确保 nxs runtime 已明确配置；不会下载或回退到其他路径。
func (i *RuntimeInspector) Ensure() (Status, error) {
	status := i.Status()
	if status.Available {
		return status, nil
	}
	return status, errorForStatus(status)
}

func (i *RuntimeInspector) withDefaults() *RuntimeInspector {
	if i == nil {
		return NewRuntimeInspector()
	}
	result := *i
	if result.goos == "" {
		result.goos = runtime.GOOS
	}
	if result.getenv == nil {
		result.getenv = os.Getenv
	}
	if result.stat == nil {
		result.stat = os.Stat
	}
	return &result
}

func (i *RuntimeInspector) statusFromEnv() (Status, bool) {
	path := strings.TrimSpace(i.getenv(commandPathEnvName))
	if path == "" {
		return Status{}, false
	}
	if i.isExecutable(path) {
		return Status{
			Available:   true,
			Path:        filepath.Clean(path),
			Source:      RuntimeSourceEnv,
			CanDownload: false,
		}, true
	}
	return Status{
		Available:   false,
		Path:        filepath.Clean(path),
		Source:      RuntimeSourceEnv,
		CanDownload: false,
		Error:       StatusErrorEnvNotExecutable,
	}, true
}

func (i *RuntimeInspector) isExecutable(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	info, err := i.stat(path)
	if err != nil || info == nil || info.IsDir() || info.Size() <= 0 {
		return false
	}
	if runtime.GOOS == "windows" || i.goos == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

func errorForStatus(status Status) error {
	switch status.Error {
	case StatusErrorEnvNotExecutable:
		return errors.New("nxs runtime env command is not executable")
	default:
		return errors.New("nxs runtime command path is not configured")
	}
}
