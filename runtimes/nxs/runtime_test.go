package nxs

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func TestRuntimePathForSelectsLatestCompatibleRuntimeFromStableChannel(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv(runtimeCacheDirEnvName, cacheDir)

	compatibleRuntime := []byte("#!/bin/sh\nexit 91\n")
	compatibleArchive := tarGzipRuntimeForTest(t, "nxs", compatibleRuntime)
	compatibleDigest := sha256HexForTest(compatibleArchive)
	newerArchive := tarGzipRuntimeForTest(t, "nxs", []byte("#!/bin/sh\nexit 92\n"))
	newerDigest := sha256HexForTest(newerArchive)
	olderArchive := tarGzipRuntimeForTest(t, "nxs", []byte("#!/bin/sh\nexit 90\n"))
	olderDigest := sha256HexForTest(olderArchive)
	compatibleHits := 0
	newerHits := 0
	olderHits := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nxs-stable/nxs-manifest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"schema_version": 1,
				"channel": "stable",
				"runtimes": [
					{
						"version": "0.9.2",
						"release_tag": "nxs-v0.9.2",
						"min_bridge_version": "9.0.0",
						"assets": [{
							"goos": "linux",
							"goarch": "amd64",
							"filename": "nxs-v0.9.2-linux-amd64.tar.gz",
							"url": "%[1]s/nxs-v0.9.2-linux-amd64.tar.gz",
							"sha256": "%[2]s",
							"archive": "tar.gz"
						}]
					},
					{
						"version": "0.9.1",
						"release_tag": "nxs-v0.9.1",
						"min_bridge_version": "0.1.6",
						"assets": [{
							"goos": "linux",
							"goarch": "amd64",
							"filename": "nxs-v0.9.1-linux-amd64.tar.gz",
							"url": "%[1]s/nxs-v0.9.1-linux-amd64.tar.gz",
							"sha256": "%[3]s",
							"archive": "tar.gz"
						}]
					},
					{
						"version": "0.9.0",
						"release_tag": "nxs-v0.9.0",
						"min_bridge_version": "0.1.0",
						"assets": [{
							"goos": "linux",
							"goarch": "amd64",
							"filename": "nxs-v0.9.0-linux-amd64.tar.gz",
							"url": "%[1]s/nxs-v0.9.0-linux-amd64.tar.gz",
							"sha256": "%[4]s",
							"archive": "tar.gz"
						}]
					}
				]
			}`, server.URL, newerDigest, compatibleDigest, olderDigest)
		case "/nxs-v0.9.2-linux-amd64.tar.gz":
			newerHits++
			_, _ = w.Write(newerArchive)
		case "/nxs-v0.9.1-linux-amd64.tar.gz":
			compatibleHits++
			_, _ = w.Write(compatibleArchive)
		case "/nxs-v0.9.0-linux-amd64.tar.gz":
			olderHits++
			_, _ = w.Write(olderArchive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(runtimeManifestURLEnvName, server.URL+"/nxs-stable/nxs-manifest.json")

	path, err := RuntimePathFor("linux", "amd64")
	if err != nil {
		t.Fatalf("RuntimePathFor() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read selected runtime: %v", err)
	}
	if string(content) != string(compatibleRuntime) {
		t.Fatalf("runtime content = %q, want latest compatible runtime", content)
	}
	if compatibleHits != 1 || newerHits != 0 || olderHits != 0 {
		t.Fatalf("downloads compatible=%d newer=%d older=%d, want only compatible", compatibleHits, newerHits, olderHits)
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

func TestRuntimePathForRejectsIncompatibleSingleManifest(t *testing.T) {
	archiveBytes := tarGzipRuntimeForTest(t, "nxs", []byte("#!/bin/sh\nexit 0\n"))
	archiveDigest := sha256HexForTest(archiveBytes)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"schema_version": 1,
			"version": "0.9.0",
			"release_tag": "nxs-v0.9.0",
			"min_bridge_version": "9.0.0",
			"assets": [{
				"goos": "linux",
				"goarch": "amd64",
				"filename": "nxs-v0.9.0-linux-amd64.tar.gz",
				"url": "nxs-v0.9.0-linux-amd64.tar.gz",
				"sha256": "` + archiveDigest + `",
				"archive": "tar.gz"
			}]
		}`))
	}))
	defer server.Close()
	t.Setenv(runtimeManifestURLEnvName, server.URL)

	_, err := RuntimePathFor("linux", "amd64")
	if err == nil || !strings.Contains(err.Error(), "requires bridge >= 9.0.0") {
		t.Fatalf("RuntimePathFor() error = %v, want bridge compatibility error", err)
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

func TestRuntimeManifestURLDefaultsToStableChannel(t *testing.T) {
	t.Setenv(runtimeManifestURLEnvName, "")
	t.Setenv(runtimeReleaseEnvName, "")

	got := runtimeManifestURL()
	if !strings.HasSuffix(got, "/nxs-stable/nxs-manifest.json") {
		t.Fatalf("runtimeManifestURL() = %q, want nxs stable manifest", got)
	}
}

func TestRuntimeManifestURLNormalizesVersionReleaseOverride(t *testing.T) {
	t.Setenv(runtimeManifestURLEnvName, "")
	t.Setenv(runtimeReleaseEnvName, "0.9.0")

	got := runtimeManifestURL()
	if !strings.HasSuffix(got, "/nxs-v0.9.0/nxs-manifest.json") {
		t.Fatalf("runtimeManifestURL() = %q, want normalized release manifest", got)
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
