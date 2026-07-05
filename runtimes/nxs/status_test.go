package nxs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeInspectorStatusEnvCommandPath(t *testing.T) {
	runtimePath := writeRuntimeExecutableForTest(t, t.TempDir(), "nxs")
	brokenPath := filepath.Join(t.TempDir(), "nxs")
	tests := []struct {
		name      string
		path      string
		available bool
		source    RuntimeSource
		err       StatusError
	}{
		{name: "available", path: runtimePath, available: true, source: RuntimeSourceEnv},
		{name: "broken", path: brokenPath, source: RuntimeSourceEnv, err: StatusErrorEnvNotExecutable},
		{name: "missing", err: StatusErrorNotFound},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(commandPathEnvName, test.path)

			status := NewRuntimeInspector(WithPlatform("linux", "amd64")).Status()
			if status.Available != test.available || status.CanDownload || status.Source != test.source ||
				status.Error != test.err {
				t.Fatalf("Status() = %+v, want available=%v source=%v error=%v", status, test.available, test.source, test.err)
			}
			if test.available && status.Path != runtimePath {
				t.Fatalf("Status().Path = %q, want %q", status.Path, runtimePath)
			}
		})
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
