package nxs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimePathForDownloadsAndCachesManifestAsset(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv(runtimeCacheDirEnvName, cacheDir)

	archiveBytes := tarGzipRuntimeForTest(t, "nxs", []byte("#!/bin/sh\nexit 0\n"))
	archiveDigest := sha256HexForTest(archiveBytes)
	assetHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nxs-manifest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"schema_version": 1,
				"version": "0.9.0",
				"release_tag": "nxs-v0.9.0",
				"assets": [{
					"goos": "linux",
					"goarch": "amd64",
					"filename": "nxs-v0.9.0-linux-amd64.tar.gz",
					"url": "nxs-v0.9.0-linux-amd64.tar.gz",
					"sha256": "` + archiveDigest + `",
					"archive": "tar.gz"
				}]
			}`))
		case "/nxs-v0.9.0-linux-amd64.tar.gz":
			assetHits++
			w.Header().Set("Content-Type", "application/gzip")
			_, _ = w.Write(archiveBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(runtimeManifestURLEnvName, server.URL+"/nxs-manifest.json")

	path, err := RuntimePathFor("linux", "amd64")
	if err != nil {
		t.Fatalf("RuntimePathFor() error = %v", err)
	}
	if !strings.HasPrefix(path, cacheDir) {
		t.Fatalf("runtime path = %q, want under %q", path, cacheDir)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("materialized runtime missing: %v", err)
	}
	if string(content) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("runtime content = %q", content)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0111 == 0 {
			t.Fatalf("runtime mode = %v err=%v, want executable", info, err)
		}
	}

	if _, err := RuntimePathFor("linux", "amd64"); err != nil {
		t.Fatalf("second RuntimePathFor() error = %v", err)
	}
	if assetHits != 1 {
		t.Fatalf("asset downloads = %d, want 1", assetHits)
	}
}

func TestRuntimePathForReportsMissingPlatform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version":1,"version":"0.9.0","assets":[{"goos":"linux","goarch":"amd64","url":"nxs.tar.gz","sha256":"abc"}]}`))
	}))
	defer server.Close()
	t.Setenv(runtimeManifestURLEnvName, server.URL)

	if _, err := RuntimePathFor("freebsd", "amd64"); err == nil {
		t.Fatal("RuntimePathFor() succeeded for missing platform")
	}
}

func TestRuntimePathForRejectsDigestMismatch(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv(runtimeCacheDirEnvName, cacheDir)

	archiveBytes := tarGzipRuntimeForTest(t, "nxs.exe", []byte("fake exe"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest":
			_, _ = w.Write([]byte(`{"schema_version":1,"version":"0.9.0","assets":[{"goos":"windows","goarch":"amd64","filename":"nxs.zip","url":"/nxs.zip","sha256":"0000","archive":"zip"}]}`))
		case "/nxs.zip":
			_, _ = w.Write(archiveBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(runtimeManifestURLEnvName, server.URL+"/manifest")

	if _, err := RuntimePathFor("windows", "amd64"); err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("RuntimePathFor() error = %v, want sha256 mismatch", err)
	}
}

func tarGzipRuntimeForTest(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatalf("write tar content: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}

func sha256HexForTest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
