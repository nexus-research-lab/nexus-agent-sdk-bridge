//go:build windows

// INPUT: 产品可信发布摘要与机器固定映像路径。
// OUTPUT: 无重解析、禁止共享写删除的实际映像/祖先owner及真实进程身份。
// POS: 不加载工作区helper，不由调用方指定任意可执行文件。
package windowssandbox

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

var nativeKernel = windows.NewLazySystemDLL("kernel32.dll")

type nativeImage struct {
	handles []windows.Handle
	file    *os.File
	path    string
}

// trustedMachineSID 仅信任机器管理员、SYSTEM和TrustedInstaller的修改权。
func trustedMachineSID(s string) bool {
	return s == "S-1-5-18" || s == "S-1-5-32-544" || s == "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
}
func verifyNativeProtected(h windows.Handle, kind windows.SE_OBJECT_TYPE, leaf bool) error {
	sd, err := windows.GetSecurityInfo(h, kind, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !trustedMachineSID(owner.String()) {
		return errors.New("machine object has untrusted owner")
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return errors.New("machine object lacks a DACL")
	}
	header := unsafe.Slice((*byte)(unsafe.Pointer(acl)), 8)
	body := unsafe.Slice((*byte)(unsafe.Pointer(acl)), int(binary.LittleEndian.Uint16(header[2:4])))
	if len(body) < 8 {
		return errors.New("invalid machine ACL")
	}
	offset := 8
	for count := uint16(0); count < acl.AceCount; count++ {
		if offset+8 > len(body) {
			return errors.New("truncated machine ACL")
		}
		size := int(binary.LittleEndian.Uint16(body[offset+2:]))
		if size < 8 || offset+size > len(body) {
			return errors.New("invalid machine ACE")
		}
		ace := body[offset : offset+size]
		offset += size
		if ace[1]&8 != 0 || ace[0] == 1 {
			continue
		}
		if ace[0] != 0 || len(ace) < 16 || ace[8] != 1 || len(ace) != 16+int(ace[9])*4 {
			return errors.New("unsupported machine ACE")
		}
		mask := binary.LittleEndian.Uint32(ace[4:8])
		danger := uint32(0x10000 | 0x40000 | 0x80000 | 0x40 | 0x100 | 0x10 | 0x40000000 | 0x10000000)
		if leaf || kind == windows.SE_REGISTRY_KEY {
			danger |= 0x2 | 0x4
		}
		if mask&danger == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace[8]))
		if !sid.IsValid() || !trustedMachineSID(sid.String()) {
			return errors.New("machine object grants mutation to untrusted principal")
		}
	}
	return nil
}

// openNativeImage 从卷根到固定映像逐级pin，可信摘要来自产品发布清单而非目标文件自身。
func openNativeImage(config Config) (*nativeImage, error) {
	o := &nativeImage{}
	root, err := windows.KnownFolderPath(windows.FOLDERID_ProgramFiles, 0)
	if err != nil {
		return o, err
	}
	o.path = filepath.Join(root, "Nexus", "SandboxRuntime", "nxs.exe")
	if !strings.EqualFold(filepath.Clean(config.HelperPath), o.path) {
		return o, errors.New("helper path differs from fixed machine installation")
	}
	expected, err := hex.DecodeString(config.HelperSHA256)
	if err != nil || len(expected) != 32 || hex.EncodeToString(expected) != config.HelperSHA256 || bytes.Equal(expected, make([]byte, 32)) {
		return o, errors.New("helper release digest is invalid")
	}
	o, err = openNativeProtectedFile(o.path)
	if err != nil {
		return o, err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(o.file, (256<<20)+1))
	if err != nil || n == 0 || n > 256<<20 || !bytes.Equal(hash.Sum(nil), expected) {
		return o, errors.Join(errors.New("machine helper differs from trusted release digest"), err)
	}
	return o, nil
}

// openNativeProtectedFile 仅供固定机器映像/manifest调用，复用实际祖先和叶pin。
func openNativeProtectedFile(path string) (*nativeImage, error) {
	o := &nativeImage{path: path}
	volume := filepath.VolumeName(o.path)
	if len(volume) != 2 || volume[1] != ':' {
		return o, errors.New("machine helper requires a local volume")
	}
	volumeName, _ := windows.UTF16PtrFromString(volume + `\`)
	if windows.GetDriveType(volumeName) != windows.DRIVE_FIXED {
		return o, errors.New("helper is not on a fixed local volume")
	}
	parts := strings.Split(strings.TrimPrefix(o.path, volume+`\`), `\`)
	currentPath := volume + `\`
	for index := -1; index < len(parts); index++ {
		if index >= 0 {
			currentPath = filepath.Join(currentPath, parts[index])
		}
		leaf := index == len(parts)-1
		name, _ := windows.UTF16PtrFromString(currentPath)
		access := uint32(windows.READ_CONTROL | windows.FILE_READ_ATTRIBUTES)
		if leaf {
			access |= windows.GENERIC_READ
		}
		h, err := windows.CreateFile(name, access, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			return o, err
		}
		o.handles = append(o.handles, h)
		var info windows.ByHandleFileInformation
		if err = windows.GetFileInformationByHandle(h, &info); err != nil {
			return o, err
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0) != leaf || leaf && info.NumberOfLinks != 1 {
			return o, errors.New("machine image path type or hardlinks changed")
		}
		if err = verifyNativeProtected(h, windows.SE_FILE_OBJECT, leaf); err != nil {
			return o, err
		}
	}
	h := o.handles[len(o.handles)-1]
	o.file = os.NewFile(uintptr(h), o.path)
	if o.file == nil {
		return o, errors.New("wrap machine image handle")
	}
	o.handles = o.handles[:len(o.handles)-1]
	return o, nil
}
func (o *nativeImage) Close() error {
	if o == nil {
		return nil
	}
	if o.file != nil {
		if err := o.file.Close(); err != nil {
			return err
		}
		o.file = nil
	}
	for index := len(o.handles) - 1; index >= 0; index-- {
		if o.handles[index] != 0 {
			if err := windows.CloseHandle(o.handles[index]); err != nil {
				return err
			}
			o.handles[index] = 0
		}
	}
	return nil
}

func nativeCreated(process windows.Handle) (uint64, error) {
	var creation, exit, kernel, user windows.Filetime
	err := windows.GetProcessTimes(process, &creation, &exit, &kernel, &user)
	return uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime), err
}
func (m *nativeMachine) verifyAccount(process windows.Handle) (result error) {
	m.tokenMu.Lock()
	defer m.tokenMu.Unlock()
	if len(m.tokens) > 0 {
		return errors.New("helper token cleanup remains pending")
	}
	expected := m.hostSID
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer func() {
		if err := token.Close(); err != nil {
			m.tokens = append(m.tokens, token)
			result = errors.Join(result, err)
		}
	}()
	user, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	if expected != "" && user.User.Sid.String() != expected {
		return errors.New("helper account differs from installed Host")
	}
	var app uint32
	var size uint32
	if err = windows.GetTokenInformation(token, windows.TokenIsAppContainer, (*byte)(unsafe.Pointer(&app)), 4, &size); err != nil || app != 0 {
		return errors.New("machine Host cannot be an AppContainer")
	}
	restricted, _, _ := windows.NewLazySystemDLL("advapi32.dll").NewProc("IsTokenRestricted").Call(uintptr(token))
	if restricted != 0 {
		return errors.New("machine Host cannot use a restricted token")
	}
	return nil
}
func (m *nativeMachine) installedHost() (host string, result error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Nexus\SandboxSystem`, registry.READ|registry.WOW64_64KEY)
	if err != nil {
		return "", err
	}
	m.configuration = key
	defer func() {
		if err := key.Close(); err != nil {
			result = errors.Join(result, err)
		} else {
			m.configuration = 0
		}
	}()
	if err = verifyNativeProtected(windows.Handle(key), windows.SE_REGISTRY_KEY, true); err != nil {
		return "", err
	}
	sid, _, err := key.GetStringValue("HostSID")
	if err != nil {
		return "", err
	}
	parsed, err := windows.StringToSid(sid)
	if err != nil || parsed.String() != sid || !strings.HasPrefix(sid, "S-1-5-21-") {
		return "", errors.New("invalid installed Host identity")
	}
	return sid, nil
}

// verifyImageProcess 保持映像pin期间核对实际进程映像；原路径不可被共享写/删除替换。
func (o *nativeImage) verifyProcess(process windows.Handle) error {
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(process, 0, &buffer[0], &size); err != nil {
		return err
	}
	if !strings.EqualFold(windows.UTF16ToString(buffer[:size]), o.path) {
		return errors.New("helper process image differs from fixed machine image")
	}
	return nil
}
