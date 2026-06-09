package nxs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeInspectorStatusUsesEnvCommandPath(t *testing.T) {
	runtimePath := writeRuntimeExecutableForTest(t, t.TempDir(), "nxs")
	t.Setenv(commandPathEnvName, runtimePath)

	status := NewRuntimeInspector(WithPlatform("linux", "amd64")).Status()
	if !status.Available || status.Path != runtimePath || status.Source != RuntimeSourceEnv {
		t.Fatalf("Status() = %+v, want env runtime", status)
	}
}

func TestRuntimeInspectorStatusRejectsBrokenEnvCommandPath(t *testing.T) {
	brokenPath := filepath.Join(t.TempDir(), "nxs")
	t.Setenv(commandPathEnvName, brokenPath)

	status := NewRuntimeInspector(WithPlatform("linux", "amd64")).Status()
	if status.Available || status.CanDownload || status.Source != RuntimeSourceEnv ||
		status.Error != StatusErrorEnvNotExecutable {
		t.Fatalf("Status() = %+v, want broken env without download", status)
	}
}

func TestRuntimeInspectorStatusRequiresEnvCommandPath(t *testing.T) {
	t.Setenv(commandPathEnvName, "")

	status := NewRuntimeInspector(WithPlatform("linux", "amd64")).Status()
	if status.Available || status.CanDownload || status.Error != StatusErrorNotFound {
		t.Fatalf("Status() = %+v, want missing env without download", status)
	}
}

func TestRuntimeInspectorEnsureRequiresEnvCommandPath(t *testing.T) {
	t.Setenv(commandPathEnvName, "")

	status, err := NewRuntimeInspector(WithPlatform("linux", "amd64")).Ensure()
	if err == nil {
		t.Fatal("Ensure() succeeded without env command")
	}
	if status.Available || status.CanDownload || status.Error != StatusErrorNotFound {
		t.Fatalf("Ensure() = %+v, want missing env status", status)
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
