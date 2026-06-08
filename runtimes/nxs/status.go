package nxs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	commandPathEnvName = "NEXUS_NXS_COMMAND_PATH"
	appRootEnvName     = "NEXUS_APP_ROOT"
	cacheDirName       = "nexus-agent-sdk-bridge"
)

// RuntimeSource 表示 nxs runtime 路径来源。
type RuntimeSource string

const (
	RuntimeSourceEnv     RuntimeSource = "env"
	RuntimeSourceAppRoot RuntimeSource = "app_root"
	RuntimeSourceCache   RuntimeSource = "cache"
)

// StatusError 表示 nxs runtime 探测失败的机器可读原因。
type StatusError string

const (
	StatusErrorNotFound                StatusError = "not_found"
	StatusErrorEnvNotExecutable        StatusError = "env_not_executable"
	StatusErrorAppRootNotExecutable    StatusError = "app_root_not_executable"
	StatusErrorDownloadFailed          StatusError = "download_failed"
	StatusErrorDownloadedNotExecutable StatusError = "downloaded_not_executable"
)

// Status 表示 nxs runtime 在当前主机上的本地可用状态。
type Status struct {
	Available   bool          `json:"available"`
	Path        string        `json:"path,omitempty"`
	Source      RuntimeSource `json:"source,omitempty"`
	CanDownload bool          `json:"can_download"`
	Error       StatusError   `json:"error,omitempty"`
}

// RuntimeInspector 负责探测和准备 nxs runtime。
type RuntimeInspector struct {
	goos           string
	goarch         string
	appRoot        string
	getenv         func(string) string
	stat           func(string) (os.FileInfo, error)
	walkDir        func(string, fs.WalkDirFunc) error
	userCacheDir   func() (string, error)
	runtimePathFor func(string, string) (string, error)
}

// RuntimeInspectorOption 配置 RuntimeInspector。
type RuntimeInspectorOption func(*RuntimeInspector)

// NewRuntimeInspector 创建 nxs runtime 探测器。
func NewRuntimeInspector(options ...RuntimeInspectorOption) *RuntimeInspector {
	inspector := &RuntimeInspector{
		goos:           runtime.GOOS,
		goarch:         runtime.GOARCH,
		getenv:         os.Getenv,
		stat:           os.Stat,
		walkDir:        filepath.WalkDir,
		userCacheDir:   os.UserCacheDir,
		runtimePathFor: RuntimePathFor,
	}
	for _, option := range options {
		if option != nil {
			option(inspector)
		}
	}
	return inspector.withDefaults()
}

// WithAppRoot 设置 Nexus 桌面包应用根目录。
func WithAppRoot(root string) RuntimeInspectorOption {
	return func(inspector *RuntimeInspector) {
		inspector.appRoot = strings.TrimSpace(root)
	}
}

// WithPlatform 设置探测平台，主要用于测试。
func WithPlatform(goos string, goarch string) RuntimeInspectorOption {
	return func(inspector *RuntimeInspector) {
		inspector.goos = strings.TrimSpace(goos)
		inspector.goarch = strings.TrimSpace(goarch)
	}
}

// WithRuntimePathFor 设置下载解析函数，主要用于测试。
func WithRuntimePathFor(runtimePathFor func(string, string) (string, error)) RuntimeInspectorOption {
	return func(inspector *RuntimeInspector) {
		inspector.runtimePathFor = runtimePathFor
	}
}

// InspectRuntime 返回当前平台 nxs runtime 本地状态，不触发下载。
func InspectRuntime(options ...RuntimeInspectorOption) Status {
	return NewRuntimeInspector(options...).Status()
}

// EnsureRuntime 确保当前平台 nxs runtime 可用，必要时下载。
func EnsureRuntime(options ...RuntimeInspectorOption) (Status, error) {
	return NewRuntimeInspector(options...).Ensure()
}

// Status 返回 nxs runtime 本地状态，不触发下载。
func (i *RuntimeInspector) Status() Status {
	inspector := i.withDefaults()
	if status, ok := inspector.statusFromEnv(); ok {
		return status
	}
	if status, ok := inspector.statusFromAppRoot(); ok {
		return status
	}
	if status, ok := inspector.statusFromCache(); ok {
		return status
	}
	return Status{
		Available:   false,
		CanDownload: true,
		Error:       StatusErrorNotFound,
	}
}

// Ensure 确保 nxs runtime 可用，必要时通过 manifest 下载。
func (i *RuntimeInspector) Ensure() (Status, error) {
	inspector := i.withDefaults()
	status := inspector.Status()
	if status.Available {
		return status, nil
	}
	if !status.CanDownload {
		return status, errorForStatus(status)
	}
	runtimePath, err := inspector.runtimePathFor(inspector.goos, inspector.goarch)
	if err != nil {
		return Status{
			Available:   false,
			CanDownload: true,
			Error:       StatusErrorDownloadFailed,
		}, err
	}
	if !inspector.isExecutable(runtimePath) {
		return Status{
			Available:   false,
			Path:        filepath.Clean(runtimePath),
			Source:      RuntimeSourceCache,
			CanDownload: true,
			Error:       StatusErrorDownloadedNotExecutable,
		}, fmt.Errorf("downloaded nxs runtime is not executable: %s", runtimePath)
	}
	return Status{
		Available:   true,
		Path:        filepath.Clean(runtimePath),
		Source:      RuntimeSourceCache,
		CanDownload: false,
	}, nil
}

func (i *RuntimeInspector) withDefaults() *RuntimeInspector {
	if i == nil {
		return NewRuntimeInspector()
	}
	result := *i
	if result.goos == "" {
		result.goos = runtime.GOOS
	}
	if result.goarch == "" {
		result.goarch = runtime.GOARCH
	}
	if result.getenv == nil {
		result.getenv = os.Getenv
	}
	if result.stat == nil {
		result.stat = os.Stat
	}
	if result.walkDir == nil {
		result.walkDir = filepath.WalkDir
	}
	if result.userCacheDir == nil {
		result.userCacheDir = os.UserCacheDir
	}
	if result.runtimePathFor == nil {
		result.runtimePathFor = RuntimePathFor
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

func (i *RuntimeInspector) statusFromAppRoot() (Status, bool) {
	root := firstNonEmpty(i.appRoot, i.getenv(appRootEnvName))
	if root == "" {
		return Status{}, false
	}
	path := filepath.Join(root, "bin", runtimeExecutableName(i.goos))
	if !i.fileExists(path) {
		return Status{}, false
	}
	if !i.isExecutable(path) {
		return Status{
			Available:   false,
			Path:        filepath.Clean(path),
			Source:      RuntimeSourceAppRoot,
			CanDownload: false,
			Error:       StatusErrorAppRootNotExecutable,
		}, true
	}
	return Status{
		Available:   true,
		Path:        filepath.Clean(path),
		Source:      RuntimeSourceAppRoot,
		CanDownload: false,
	}, true
}

func (i *RuntimeInspector) statusFromCache() (Status, bool) {
	root, ok := i.runtimeCacheRoot()
	if !ok {
		return Status{}, false
	}
	path := i.findCachedRuntime(root)
	if path == "" {
		return Status{}, false
	}
	return Status{
		Available:   true,
		Path:        filepath.Clean(path),
		Source:      RuntimeSourceCache,
		CanDownload: false,
	}, true
}

func (i *RuntimeInspector) runtimeCacheRoot() (string, bool) {
	base := strings.TrimSpace(i.getenv(runtimeCacheDirEnvName))
	if base == "" {
		cacheDir, err := i.userCacheDir()
		if err != nil || strings.TrimSpace(cacheDir) == "" {
			return "", false
		}
		base = cacheDir
	}
	return filepath.Join(base, cacheDirName, "runtimes", "nxs"), true
}

func (i *RuntimeInspector) findCachedRuntime(root string) string {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return ""
	}
	executableName := runtimeExecutableName(i.goos)
	platform := i.goos + "-" + i.goarch
	candidates := []cachedRuntimeCandidate{}
	_ = i.walkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() || entry.Name() != executableName {
			return nil
		}
		if !i.isExecutable(path) || !runtimePathMatchesPlatform(path, platform) {
			return nil
		}
		candidates = append(candidates, cachedRuntimeCandidate{
			path:    path,
			version: cachedRuntimeVersion(root, path),
		})
		return nil
	})
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(left int, right int) bool {
		compared := compareRuntimeVersions(candidates[left].version, candidates[right].version)
		if compared == 0 {
			return candidates[left].path > candidates[right].path
		}
		return compared > 0
	})
	return candidates[0].path
}

func (i *RuntimeInspector) fileExists(path string) bool {
	info, err := i.stat(strings.TrimSpace(path))
	return err == nil && info != nil && !info.IsDir()
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

type cachedRuntimeCandidate struct {
	path    string
	version string
}

func runtimePathMatchesPlatform(runtimePath string, platform string) bool {
	parts := strings.Split(filepath.Clean(runtimePath), string(filepath.Separator))
	for _, part := range parts {
		if part == platform {
			return true
		}
	}
	return false
}

func cachedRuntimeVersion(root string, runtimePath string) string {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(runtimePath))
	if err != nil {
		return ""
	}
	segments := strings.Split(relative, string(filepath.Separator))
	if len(segments) == 0 {
		return ""
	}
	return segments[0]
}

func errorForStatus(status Status) error {
	switch status.Error {
	case StatusErrorEnvNotExecutable:
		return errors.New("nxs runtime env command is not executable")
	case StatusErrorAppRootNotExecutable:
		return errors.New("nxs runtime app root command is not executable")
	case StatusErrorDownloadedNotExecutable:
		return errors.New("downloaded nxs runtime is not executable")
	default:
		return errors.New("nxs runtime is not available")
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
