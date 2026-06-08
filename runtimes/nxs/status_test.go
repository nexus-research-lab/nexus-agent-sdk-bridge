package nxs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeInspectorStatusUsesEnvCommandPath(t *testing.T) {
	runtimePath := writeRuntimeExecutableForTest(t, t.TempDir(), "nxs")
	t.Setenv(commandPathEnvName, runtimePath)
	t.Setenv(appRootEnvName, "")

	status := NewRuntimeInspector(WithPlatform("linux", "amd64")).Status()
	if !status.Available || status.Path != runtimePath || status.Source != RuntimeSourceEnv {
		t.Fatalf("Status() = %+v, want env runtime", status)
	}
}

func TestRuntimeInspectorStatusRejectsBrokenEnvCommandPath(t *testing.T) {
	brokenPath := filepath.Join(t.TempDir(), "nxs")
	t.Setenv(commandPathEnvName, brokenPath)
	t.Setenv(appRootEnvName, "")

	status := NewRuntimeInspector(WithPlatform("linux", "amd64")).Status()
	if status.Available || status.CanDownload || status.Source != RuntimeSourceEnv ||
		status.Error != StatusErrorEnvNotExecutable {
		t.Fatalf("Status() = %+v, want broken env without download", status)
	}
}

func TestRuntimeInspectorStatusPrefersEnvOverAppRootRuntime(t *testing.T) {
	root := t.TempDir()
	_ = writeRuntimeExecutableForTest(t, filepath.Join(root, "bin"), "nxs")
	brokenPath := filepath.Join(t.TempDir(), "nxs")
	t.Setenv(commandPathEnvName, brokenPath)

	status := NewRuntimeInspector(
		WithPlatform("linux", "amd64"),
		WithAppRoot(root),
	).Status()
	if status.Available || status.Source != RuntimeSourceEnv ||
		status.Error != StatusErrorEnvNotExecutable {
		t.Fatalf("Status() = %+v, want env override to win", status)
	}
}

func TestRuntimeInspectorStatusUsesAppRootRuntime(t *testing.T) {
	root := t.TempDir()
	runtimePath := writeRuntimeExecutableForTest(t, filepath.Join(root, "bin"), "nxs")
	t.Setenv(commandPathEnvName, "")
	t.Setenv(appRootEnvName, "")

	status := NewRuntimeInspector(
		WithPlatform("linux", "amd64"),
		WithAppRoot(root),
	).Status()
	if !status.Available || status.Path != runtimePath || status.Source != RuntimeSourceAppRoot {
		t.Fatalf("Status() = %+v, want app root runtime", status)
	}
}

func TestRuntimeInspectorStatusUsesCachedRuntime(t *testing.T) {
	cacheDir := t.TempDir()
	runtimePath := writeRuntimeExecutableForTest(
		t,
		filepath.Join(cacheDir, cacheDirName, "runtimes", "nxs", "0.1.2", "linux-amd64", "digest"),
		"nxs",
	)
	t.Setenv(commandPathEnvName, "")
	t.Setenv(appRootEnvName, "")
	t.Setenv(runtimeCacheDirEnvName, cacheDir)

	status := NewRuntimeInspector(WithPlatform("linux", "amd64")).Status()
	if !status.Available || status.Path != runtimePath || status.Source != RuntimeSourceCache {
		t.Fatalf("Status() = %+v, want cached runtime", status)
	}
}

func TestRuntimeInspectorEnsureUsesRuntimeResolver(t *testing.T) {
	runtimePath := writeRuntimeExecutableForTest(t, t.TempDir(), "nxs")
	t.Setenv(commandPathEnvName, "")
	t.Setenv(appRootEnvName, "")
	t.Setenv(runtimeCacheDirEnvName, t.TempDir())

	status, err := NewRuntimeInspector(
		WithPlatform("linux", "amd64"),
		WithRuntimePathFor(func(goos string, goarch string) (string, error) {
			if goos != "linux" || goarch != "amd64" {
				t.Fatalf("runtimePathFor(%q, %q), want linux/amd64", goos, goarch)
			}
			return runtimePath, nil
		}),
	).Ensure()
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if !status.Available || status.Path != runtimePath || status.Source != RuntimeSourceCache {
		t.Fatalf("Ensure() = %+v, want downloaded runtime", status)
	}
}

func TestRuntimeInspectorEnsureBlocksBrokenEnvCommandPath(t *testing.T) {
	brokenPath := filepath.Join(t.TempDir(), "nxs")
	t.Setenv(commandPathEnvName, brokenPath)

	status, err := NewRuntimeInspector(
		WithPlatform("linux", "amd64"),
		WithRuntimePathFor(func(string, string) (string, error) {
			return "", errors.New("should not download")
		}),
	).Ensure()
	if err == nil {
		t.Fatal("Ensure() succeeded for broken env command")
	}
	if status.Available || status.CanDownload || status.Error != StatusErrorEnvNotExecutable {
		t.Fatalf("Ensure() = %+v, want broken env status", status)
	}
}

func writeRuntimeExecutableForTest(t *testing.T, directory string, name string) string {
	t.Helper()
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("创建 runtime 目录失败: %v", err)
	}
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("写入 runtime 失败: %v", err)
	}
	return path
}
