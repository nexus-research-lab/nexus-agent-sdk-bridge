package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/mcpwire"
)

const runtimeArgFileMaxAge = 24 * time.Hour

var runtimeArgFilesRoot = defaultRuntimeArgFilesRoot

func defaultRuntimeArgFilesRoot(env map[string]string) string {
	return filepath.Join(resolveConfigDir(env), "runtime", "arg-files")
}

func materializeProcessArgFiles(options *Options) error {
	return materializeProcessArgFilesForOS(runtime.GOOS, options)
}

func materializeProcessArgFilesForOS(goos string, options *Options) error {
	if goos != "windows" || options == nil || options.Transport != nil || options.DirectConnect != nil {
		return nil
	}
	if err := cleanupRuntimeArgFiles(options.Env); err != nil {
		return err
	}
	if options.System.Append != "" {
		path, err := writeRuntimeArgFile(options.Env, "append-system-prompt", ".txt", []byte(options.System.Append))
		if err != nil {
			return fmt.Errorf("write append system prompt arg file: %w", err)
		}
		if options.ExtraArgs == nil {
			options.ExtraArgs = map[string]string{}
		}
		options.ExtraArgs["append-system-prompt-file"] = path
		options.System.Append = ""
	}
	if len(options.resolvedMCPServers()) > 0 && strings.TrimSpace(options.MCP.Config) == "" {
		payload, sdkServers, err := mcpwire.MarshalConfig(options.resolvedMCPServers())
		if err != nil {
			return err
		}
		path, err := writeRuntimeArgFile(options.Env, "mcp-config", ".json", payload)
		if err != nil {
			return fmt.Errorf("write MCP config arg file: %w", err)
		}
		options.MCP.Config = path
		options.MCP.Servers = nil
		options.MCP.SDKServers = sdkServers
	}
	return nil
}

func cleanupRuntimeArgFiles(env map[string]string) error {
	root := runtimeArgFilesRoot(env)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return fmt.Errorf("create runtime arg file dir: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	expiredBefore := time.Now().Add(-runtimeArgFileMaxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(expiredBefore) {
			continue
		}
		_ = os.Remove(filepath.Join(root, entry.Name()))
	}
	return nil
}

func writeRuntimeArgFile(env map[string]string, prefix string, extension string, payload []byte) (string, error) {
	root := runtimeArgFilesRoot(env)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(root, fmt.Sprintf("%s-%s%s", prefix, runtimeArgFileDigest(payload), extension))
	if sameRuntimeArgFile(path, payload) {
		return path, nil
	}
	temp, err := os.CreateTemp(root, "."+prefix+"-*"+extension)
	if err != nil {
		return "", err
	}
	tempPath := temp.Name()
	if err := os.Chmod(tempPath, 0o600); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return "", err
	}
	if _, err := temp.Write(payload); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func sameRuntimeArgFile(path string, payload []byte) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(data, payload)
}

func runtimeArgFileDigest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])[:16]
}
