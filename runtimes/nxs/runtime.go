package nxs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	runtimeCacheDirEnvName    = "NEXUS_NXS_RUNTIME_CACHE_DIR"
	runtimeReleaseEnvName     = "NEXUS_NXS_RUNTIME_RELEASE"
	runtimeManifestURLEnvName = "NEXUS_NXS_RUNTIME_MANIFEST_URL"

	defaultRuntimeReleaseTag       = "nxs-stable"
	bootstrapRuntimeReleaseTag     = "nxs-v0.1.2"
	bridgeDevelopmentVersion       = "0.1.7"
	bridgeModulePath               = "github.com/nexus-research-lab/nexus-agent-sdk-bridge"
	bridgeReleaseBaseURL           = "https://github.com/nexus-research-lab/nexus-agent-sdk-bridge/releases/download"
	defaultRuntimeManifestName     = "nxs-manifest.json"
	defaultRuntimeReleaseTagPrefix = "nxs-v"
)

type runtimeManifest struct {
	SchemaVersion    int                    `json:"schema_version"`
	Channel          string                 `json:"channel,omitempty"`
	Version          string                 `json:"version,omitempty"`
	ReleaseTag       string                 `json:"release_tag,omitempty"`
	MinBridgeVersion string                 `json:"min_bridge_version,omitempty"`
	Assets           []runtimeAssetManifest `json:"assets,omitempty"`
	Runtimes         []runtimeManifest      `json:"runtimes,omitempty"`
}

type runtimeAssetManifest struct {
	GOOS     string `json:"goos"`
	GOARCH   string `json:"goarch"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Archive  string `json:"archive"`
}

// RuntimePath 返回当前平台可执行 nxs runtime 的本地文件路径。
func RuntimePath() (string, error) {
	return RuntimePathFor(runtime.GOOS, runtime.GOARCH)
}

// RuntimePathFor 返回指定平台可执行 nxs runtime 的本地文件路径。
func RuntimePathFor(goos string, goarch string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return runtimePathFor(ctx, goos, goarch, runtimeManifestURL())
}

func runtimePathFor(ctx context.Context, goos string, goarch string, manifestURL string) (string, error) {
	platform := goos + "-" + goarch
	executableName := runtimeExecutableName(goos)
	manifest, err := loadCompatibleRuntimeManifest(ctx, manifestURL, goos, goarch)
	if err != nil {
		if manifestURL != defaultRuntimeManifestURL() {
			return "", err
		}
		manifest, err = loadCompatibleRuntimeManifest(ctx, runtimeReleaseManifestURL(bootstrapRuntimeReleaseTag), goos, goarch)
		if err != nil {
			return "", err
		}
	}
	asset, err := selectRuntimeAsset(manifest, goos, goarch)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(asset.SHA256) == "" {
		return "", fmt.Errorf("nxs runtime asset %s has no sha256", asset.Filename)
	}
	outputPath, err := materializedRuntimePath(platform, manifestVersion(manifest), executableName, asset.SHA256)
	if err != nil {
		return "", err
	}
	if usableRuntimeFile(outputPath) {
		return outputPath, nil
	}

	assetURL, err := resolveRuntimeAssetURL(manifestURL, asset.URL)
	if err != nil {
		return "", err
	}
	archiveBytes, err := downloadRuntimeBytes(ctx, assetURL)
	if err != nil {
		return "", err
	}
	if err := verifySHA256(archiveBytes, asset.SHA256); err != nil {
		return "", err
	}
	runtimeBytes, err := extractRuntimeExecutable(archiveBytes, archiveKind(asset), executableName)
	if err != nil {
		return "", err
	}
	if err := ensureRuntimeFile(outputPath, runtimeBytes); err != nil {
		return "", err
	}
	return outputPath, nil
}

func runtimeManifestURL() string {
	if override := strings.TrimSpace(os.Getenv(runtimeManifestURLEnvName)); override != "" {
		return override
	}
	releaseTag := strings.TrimSpace(os.Getenv(runtimeReleaseEnvName))
	if releaseTag == "" {
		releaseTag = defaultRuntimeReleaseTag
	} else {
		releaseTag = normalizeRuntimeReleaseTag(releaseTag)
	}
	return runtimeReleaseManifestURL(releaseTag)
}

func defaultRuntimeManifestURL() string {
	return runtimeReleaseManifestURL(defaultRuntimeReleaseTag)
}

func runtimeReleaseManifestURL(releaseTag string) string {
	return strings.TrimRight(bridgeReleaseBaseURL, "/") + "/" + releaseTag + "/" + defaultRuntimeManifestName
}

func loadCompatibleRuntimeManifest(ctx context.Context, manifestURL string, goos string, goarch string) (runtimeManifest, error) {
	manifest, err := fetchRuntimeManifest(ctx, manifestURL)
	if err != nil {
		return runtimeManifest{}, err
	}
	manifest, err = selectCompatibleRuntimeManifest(manifest, currentBridgeVersion(), goos, goarch)
	if err != nil {
		return runtimeManifest{}, err
	}
	return manifest, nil
}

func fetchRuntimeManifest(ctx context.Context, manifestURL string) (runtimeManifest, error) {
	data, err := downloadRuntimeBytes(ctx, manifestURL)
	if err != nil {
		return runtimeManifest{}, fmt.Errorf("download nxs runtime manifest: %w", err)
	}
	var manifest runtimeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return runtimeManifest{}, fmt.Errorf("decode nxs runtime manifest: %w", err)
	}
	if len(manifest.Assets) == 0 && len(manifest.Runtimes) == 0 {
		return runtimeManifest{}, errors.New("nxs runtime manifest has no assets or runtimes")
	}
	return manifest, nil
}

func selectCompatibleRuntimeManifest(manifest runtimeManifest, bridgeVersion string, goos string, goarch string) (runtimeManifest, error) {
	if len(manifest.Runtimes) == 0 {
		if !runtimeSupportsBridge(manifest, bridgeVersion) {
			return runtimeManifest{}, incompatibleRuntimeError(manifest, bridgeVersion)
		}
		return manifest, nil
	}

	candidates := make([]runtimeManifest, 0, len(manifest.Runtimes))
	for _, candidate := range manifest.Runtimes {
		if !runtimeSupportsBridge(candidate, bridgeVersion) {
			continue
		}
		if _, err := selectRuntimeAsset(candidate, goos, goarch); err != nil {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		channel := strings.TrimSpace(manifest.Channel)
		if channel == "" {
			channel = manifestRuntimeLabel(manifest)
		}
		return runtimeManifest{}, fmt.Errorf("nxs runtime channel %s has no compatible runtime for bridge %s on %s-%s", channel, bridgeVersion, goos, goarch)
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		return compareRuntimeVersions(candidates[i].Version, candidates[j].Version) > 0
	})
	return candidates[0], nil
}

func selectRuntimeAsset(manifest runtimeManifest, goos string, goarch string) (runtimeAssetManifest, error) {
	for _, asset := range manifest.Assets {
		if asset.GOOS == goos && asset.GOARCH == goarch {
			if strings.TrimSpace(asset.URL) == "" {
				return runtimeAssetManifest{}, fmt.Errorf("nxs runtime asset %s-%s has no url", goos, goarch)
			}
			return asset, nil
		}
	}
	return runtimeAssetManifest{}, fmt.Errorf("nxs runtime asset %s-%s is not available", goos, goarch)
}

func runtimeSupportsBridge(manifest runtimeManifest, bridgeVersion string) bool {
	minVersion := strings.TrimSpace(manifest.MinBridgeVersion)
	if minVersion == "" {
		return true
	}
	return compareRuntimeVersions(bridgeVersion, minVersion) >= 0
}

func incompatibleRuntimeError(manifest runtimeManifest, bridgeVersion string) error {
	return fmt.Errorf("nxs runtime %s requires bridge >= %s, current bridge %s", manifestRuntimeLabel(manifest), manifest.MinBridgeVersion, bridgeVersion)
}

func manifestRuntimeLabel(manifest runtimeManifest) string {
	if releaseTag := strings.TrimSpace(manifest.ReleaseTag); releaseTag != "" {
		return releaseTag
	}
	if version := strings.TrimSpace(manifest.Version); version != "" {
		return version
	}
	if channel := strings.TrimSpace(manifest.Channel); channel != "" {
		return channel
	}
	return "unknown"
}

func normalizeRuntimeReleaseTag(releaseTag string) string {
	tag := strings.TrimSpace(releaseTag)
	if strings.HasPrefix(tag, "nxs-") {
		return tag
	}
	version := strings.TrimPrefix(tag, "v")
	if version == tag && !strings.Contains(tag, ".") {
		return tag
	}
	return defaultRuntimeReleaseTagPrefix + version
}

func currentBridgeVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return bridgeDevelopmentVersion
	}
	if buildInfo.Main.Path == bridgeModulePath && stableBuildVersion(buildInfo.Main.Version) != "" {
		return stableBuildVersion(buildInfo.Main.Version)
	}
	for _, dependency := range buildInfo.Deps {
		if dependency.Path != bridgeModulePath {
			continue
		}
		if version := stableBuildVersion(dependency.Version); version != "" {
			return version
		}
	}
	return bridgeDevelopmentVersion
}

func stableBuildVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" || trimmed == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(trimmed, "v")
}

func compareRuntimeVersions(left string, right string) int {
	leftVersion := parseRuntimeVersion(left)
	rightVersion := parseRuntimeVersion(right)
	if !leftVersion.valid || !rightVersion.valid {
		return strings.Compare(left, right)
	}
	for index := 0; index < len(leftVersion.parts); index++ {
		if leftVersion.parts[index] > rightVersion.parts[index] {
			return 1
		}
		if leftVersion.parts[index] < rightVersion.parts[index] {
			return -1
		}
	}
	switch {
	case leftVersion.prerelease == rightVersion.prerelease:
		return 0
	case leftVersion.prerelease == "":
		return 1
	case rightVersion.prerelease == "":
		return -1
	default:
		return strings.Compare(leftVersion.prerelease, rightVersion.prerelease)
	}
}

type runtimeVersion struct {
	parts      [3]int
	prerelease string
	valid      bool
}

func parseRuntimeVersion(value string) runtimeVersion {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "nxs-")
	trimmed = strings.TrimPrefix(trimmed, "v")
	core, prerelease, _ := strings.Cut(trimmed, "-")
	segments := strings.Split(core, ".")
	if len(segments) != 3 {
		return runtimeVersion{}
	}
	var version runtimeVersion
	for index, segment := range segments {
		parsed, err := strconv.Atoi(segment)
		if err != nil {
			return runtimeVersion{}
		}
		version.parts[index] = parsed
	}
	version.prerelease = prerelease
	version.valid = true
	return version
}

func resolveRuntimeAssetURL(manifestURL string, assetURL string) (string, error) {
	parsedAssetURL, err := url.Parse(strings.TrimSpace(assetURL))
	if err != nil {
		return "", fmt.Errorf("parse nxs runtime asset url: %w", err)
	}
	if parsedAssetURL.IsAbs() {
		return parsedAssetURL.String(), nil
	}
	parsedManifestURL, err := url.Parse(manifestURL)
	if err != nil {
		return "", fmt.Errorf("parse nxs runtime manifest url: %w", err)
	}
	return parsedManifestURL.ResolveReference(parsedAssetURL).String(), nil
}

func downloadRuntimeBytes(ctx context.Context, rawURL string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected http status %s", response.Status)
	}
	return io.ReadAll(response.Body)
}

func verifySHA256(data []byte, want string) error {
	digest := sha256.Sum256(data)
	got := hex.EncodeToString(digest[:])
	if !strings.EqualFold(got, strings.TrimSpace(want)) {
		return fmt.Errorf("nxs runtime sha256 mismatch: got %s, want %s", got, want)
	}
	return nil
}

func extractRuntimeExecutable(data []byte, archive string, executableName string) ([]byte, error) {
	switch archive {
	case "tar.gz", "tgz":
		return extractTarGzipRuntime(data, executableName)
	case "zip":
		return extractZipRuntime(data, executableName)
	case "raw":
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported nxs runtime archive type %q", archive)
	}
}

func extractTarGzipRuntime(data []byte, executableName string) ([]byte, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg || path.Base(header.Name) != executableName {
			continue
		}
		return io.ReadAll(reader)
	}
	return nil, fmt.Errorf("nxs executable %s not found in tar.gz", executableName)
}

func extractZipRuntime(data []byte, executableName string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || path.Base(file.Name) != executableName {
			continue
		}
		content, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(content)
		closeErr := content.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return data, nil
	}
	return nil, fmt.Errorf("nxs executable %s not found in zip", executableName)
}

func archiveKind(asset runtimeAssetManifest) string {
	archive := strings.TrimSpace(asset.Archive)
	if archive != "" {
		return archive
	}
	filename := strings.ToLower(strings.TrimSpace(asset.Filename))
	switch {
	case strings.HasSuffix(filename, ".tar.gz"):
		return "tar.gz"
	case strings.HasSuffix(filename, ".tgz"):
		return "tgz"
	case strings.HasSuffix(filename, ".zip"):
		return "zip"
	default:
		return "raw"
	}
}

func manifestVersion(manifest runtimeManifest) string {
	if version := strings.TrimSpace(manifest.Version); version != "" {
		return version
	}
	if releaseTag := strings.TrimSpace(manifest.ReleaseTag); releaseTag != "" {
		return releaseTag
	}
	return "unknown"
}

func runtimeExecutableName(goos string) string {
	if goos == "windows" {
		return "nxs.exe"
	}
	return "nxs"
}

func materializedRuntimePath(platform string, version string, executableName string, sha256Value string) (string, error) {
	cacheDir := os.Getenv(runtimeCacheDirEnvName)
	if cacheDir == "" {
		resolvedCacheDir, err := os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("resolve cache dir: %w", err)
		}
		cacheDir = resolvedCacheDir
	}
	versionDir := sanitizePathComponent(version)
	digestDir := strings.ToLower(strings.TrimSpace(sha256Value))
	if len(digestDir) > 16 {
		digestDir = digestDir[:16]
	}
	if digestDir == "" {
		digestDir = "unknown"
	}
	return filepath.Join(cacheDir, "nexus-agent-sdk-bridge", "runtimes", "nxs", versionDir, platform, digestDir, executableName), nil
}

func ensureRuntimeFile(path string, data []byte) error {
	if usableRuntimeFile(path) {
		return os.Chmod(path, 0755)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create nxs runtime cache dir: %w", err)
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0755); err != nil {
		return fmt.Errorf("write nxs runtime: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("install nxs runtime: %w", err)
	}
	return os.Chmod(path, 0755)
}

func usableRuntimeFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func sanitizePathComponent(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r == '.' || r == '-' || r == '_' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "unknown"
	}
	return result
}
