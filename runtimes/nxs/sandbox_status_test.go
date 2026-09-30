package nxs

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDecodeSandboxStatusRejectsUnprovenAvailability(t *testing.T) {
	for _, input := range []string{
		`{}`, `{"version":2,"platform":"macos","backend_supported":true,"dependencies_available":true}`,
		`{"version":1,"platform":"macos","backend_supported":true}`,
		`{"version":1,"platform":"windows","backend_supported":false,"dependencies_available":true}`,
		`{"version":1,"platform":"windows","backend_supported":false,"dependencies_available":false}`,
		`{"version":1,"platform":"macos","backend_supported":true,"dependencies_available":true,"unavailable_reason":"missing"}`,
		`{"version":1,"platform":"macos","backend_supported":true,"dependencies_available":true} {}`,
	} {
		if got, err := decodeSandboxStatus([]byte(input)); err == nil || got != nil {
			t.Fatalf("accepted invalid diagnosis: %s", input)
		}
	}
	for _, input := range []string{
		`{"version":1,"platform":"macos","backend_supported":true,"dependencies_available":true}`,
		`{"version":1,"platform":"windows","backend_supported":false,"dependencies_available":false,"unavailable_reason":"unsupported"}`,
	} {
		if _, err := decodeSandboxStatus([]byte(input)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSandboxStatusOutputBound(t *testing.T) {
	output := &sandboxStatusOutput{}
	if _, err := io.Copy(output, strings.NewReader(strings.Repeat("x", 16385))); err == nil || !output.exceeded || output.buffer.Len() > 16384 {
		t.Fatal("output limit bypassed")
	}
}

func TestSandboxStatusCommandAndCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix script fixture; protocol checks run on all platforms")
	}
	for _, scenario := range []string{"valid", "legacy", "cancel", "oversized"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nxs with spaces")
			script := "#!/bin/sh\n[ \"$#\" = 1 ] && [ \"$1\" = --sandbox-status ] || exit 9\n"
			switch scenario {
			case "valid":
				script += "printf '%s' '{\"version\":1,\"platform\":\"macos\",\"backend_supported\":true,\"dependencies_available\":true}'\n"
			case "legacy":
				script += "exit 2\n"
			case "cancel":
				script += "while :; do :; done\n"
			case "oversized":
				script += "i=0; while [ $i -lt 17000 ]; do printf x; i=$((i+1)); done\n"
			}
			if err := os.WriteFile(path, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv(commandPathEnvName, path)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, err := NewRuntimeInspector().SandboxStatus(ctx)
			if scenario == "valid" {
				if err != nil || got == nil || !got.BackendSupported {
					t.Fatalf("status: %+v %v", got, err)
				}
			} else if got != nil || err == nil {
				t.Fatalf("failed diagnostic became known: %+v %v", got, err)
			}
			if scenario == "cancel" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("cancellation lost: %v", err)
			}
			if !NewRuntimeInspector().Status().Available {
				t.Fatal("diagnostic changed runtime availability")
			}
		})
	}
}

func TestSandboxStatusRealRuntime(t *testing.T) {
	path := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if path == "" {
		t.Skip("set NEXUS_SANDBOX_TEST_BINARY to built nxs")
	}
	t.Setenv(commandPathEnvName, path)
	got, err := NewRuntimeInspector().SandboxStatus(context.Background())
	if err != nil || got == nil {
		t.Fatalf("real diagnostic: %+v %v", got, err)
	}
	t.Logf("native sandbox: %+v", got)
}
