//go:build windows

// INPUT: 固定ProgramFiles安装目录中由机器安装器保护的4KiB清单。
// OUTPUT: 严格校验清单及实际helper摘要/PE架构之后的只读安装信息。
// POS: 签名指纹只是元数据，信任来源为受保护安装根；不启动进程或修改ACL。
package windowssandbox

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type installedManifest struct {
	Version          int    `json:"version"`
	BootstrapSHA256  string `json:"bootstrapSHA256"`
	ServiceSHA256    string `json:"serviceSHA256"`
	Architecture     string `json:"architecture"`
	PackageVersion   string `json:"packageVersion"`
	SignerThumbprint string `json:"signerThumbprint"`
}

// installedCleanupError 保留关闭失败的pin，调用方可通过errors.As取得io.Closer再次收口。
type installedCleanupError struct {
	owners []*nativeImage
	cause  error
}

func (e *installedCleanupError) Error() string {
	if e.cause == nil {
		return "installed sandbox pin cleanup completed"
	}
	return "installed sandbox pin cleanup failed: " + e.cause.Error()
}
func (e *installedCleanupError) Unwrap() error { return e.cause }
func (e *installedCleanupError) Close() error {
	var errs []error
	for i, owner := range e.owners {
		if owner != nil {
			if err := owner.Close(); err != nil {
				errs = append(errs, err)
			} else {
				e.owners[i] = nil
			}
		}
	}
	e.cause = errors.Join(errs...)
	return e.cause
}

// decodeInstalledManifest 精确字段及值校验，不接受大小写别名/重复/null或签名字符串代替文件验证。
func decodeInstalledManifest(body []byte) (installedManifest, error) {
	var m installedManifest
	if len(body) == 0 || len(body) > 4096 {
		return m, errors.New("installed manifest exceeds 4KiB bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := uniqueControlValue(decoder, 0); err != nil {
		return m, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return m, errors.New("trailing installed manifest data")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || len(fields) != 6 {
		return m, errors.New("installed manifest has wrong fields")
	}
	for _, key := range []string{"version", "bootstrapSHA256", "serviceSHA256", "architecture", "packageVersion", "signerThumbprint"} {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return m, errors.New("installed manifest field is missing or null")
		}
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return m, err
	}
	if m.Version != 1 || m.Architecture != "amd64" && m.Architecture != "arm64" || m.Architecture != runtime.GOARCH {
		return m, errors.New("installed manifest version or architecture differs")
	}
	for _, digest := range []string{m.BootstrapSHA256, m.ServiceSHA256} {
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != digest || bytes.Equal(raw, make([]byte, 32)) {
			return m, errors.New("invalid installed image digest")
		}
	}
	signer, err := hex.DecodeString(m.SignerThumbprint)
	if err != nil || len(signer) != 20 || strings.ToUpper(hex.EncodeToString(signer)) != m.SignerThumbprint || bytes.Equal(signer, make([]byte, 20)) {
		return m, errors.New("invalid installed signer metadata")
	}
	parts := strings.Split(m.PackageVersion, ".")
	if len(parts) != 3 {
		return m, errors.New("invalid installed package version")
	}
	for _, part := range parts {
		number, err := strconv.ParseUint(part, 10, 16)
		if err != nil || strconv.FormatUint(number, 10) != part {
			return m, errors.New("invalid installed package version component")
		}
	}
	return m, nil
}

// LoadInstalled 在所有实际pin关闭成功前不交付配置；不创建管道、进程或安装授权。
func LoadInstalled(ctx context.Context) (installed Installed, result error) {
	if ctx == nil {
		return installed, errors.New("installation discovery requires a context")
	}
	if err := ctx.Err(); err != nil {
		return installed, err
	}
	var owners []*nativeImage
	defer func() {
		var failed []*nativeImage
		var errs []error
		for i := len(owners) - 1; i >= 0; i-- {
			if owners[i] != nil {
				if err := owners[i].Close(); err != nil {
					failed = append(failed, owners[i])
					errs = append(errs, err)
				}
			}
		}
		if len(failed) > 0 {
			result = errors.Join(result, &installedCleanupError{owners: failed, cause: errors.Join(errs...)})
		}
		result = errors.Join(result, ctx.Err())
		if result != nil {
			installed = Installed{}
		}
	}()
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil {
		return installed, err
	}
	directory := filepath.Join(root, "Nexus", "SandboxRuntime")
	manifest, err := openNativeProtectedFile(filepath.Join(directory, "sandbox-runtime.json"))
	owners = append(owners, manifest)
	if err != nil {
		return installed, err
	}
	body, err := io.ReadAll(io.LimitReader(manifest.file, 4097))
	if err != nil {
		return installed, err
	}
	m, err := decodeInstalledManifest(body)
	if err != nil {
		return installed, err
	}
	if err = ctx.Err(); err != nil {
		return installed, err
	}
	path := filepath.Join(directory, "nxs.exe")
	image, err := openNativeImage(Config{HelperPath: path, HelperSHA256: m.BootstrapSHA256})
	owners = append(owners, image)
	if err != nil {
		return installed, err
	}
	var dos [64]byte
	if _, err = image.file.ReadAt(dos[:], 0); err != nil {
		return installed, err
	}
	offset := int64(binary.LittleEndian.Uint32(dos[60:]))
	if string(dos[:2]) != "MZ" || offset < 64 || offset > (256<<20)-6 {
		return installed, errors.New("installed helper has invalid PE header")
	}
	var pe [6]byte
	if _, err = image.file.ReadAt(pe[:], offset); err != nil {
		return installed, err
	}
	machine := binary.LittleEndian.Uint16(pe[4:])
	expected := uint16(0x8664)
	if m.Architecture == "arm64" {
		expected = 0xaa64
	}
	if string(pe[:4]) != "PE\x00\x00" || machine != expected {
		return installed, errors.New("installed helper PE architecture differs from manifest")
	}
	return Installed{HelperPath: path, HelperSHA256: m.BootstrapSHA256, PackageVersion: m.PackageVersion, Architecture: m.Architecture}, nil
}
