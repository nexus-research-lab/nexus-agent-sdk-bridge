// INPUT: 可信启动参数携带的观察者身份及当前 Unix 控制连接。
// OUTPUT: 内核连接身份双向核验；不读取连接正文中的自报身份。
// POS: 引导进程认证宿主，身份不是秘密也不是可转借的执行授权。
package processscope

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net"
	"strings"
)

// ObserverIdentity 返回当前进程完整 audit token 的固定编码，仅用于启动身份绑定。
func ObserverIdentity() (string, error) {
	k := newKernel()
	if err := k.available(); err != nil {
		return "", err
	}
	self, err := k.self()
	if err != nil {
		return "", err
	}
	var encoded [32]byte
	for i, value := range self.audit {
		binary.BigEndian.PutUint32(encoded[i*4:], value)
	}
	return hex.EncodeToString(encoded[:]), nil
}

// VerifyObserver 验证连接对端是启动时指定的同用户宿主。
// expected 必须来自可信 launcher 的固定启动参数，不能来自对端发来的正文。
func VerifyObserver(conn *net.UnixConn, expected string) error {
	identity, err := decodeIdentity(expected)
	if err != nil {
		return err
	}
	if conn == nil {
		return errors.New("observer connection is required")
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var observed error
	if err := raw.Control(func(fd uintptr) { observed = verifyObserver(newKernel(), int(fd), identity) }); err != nil {
		return err
	}
	return observed
}

func decodeIdentity(value string) ([8]uint32, error) {
	var result [8]uint32
	if len(value) != 64 {
		return result, errors.New("invalid observer identity encoding")
	}
	data, err := hex.DecodeString(value)
	if err != nil {
		return result, errors.New("invalid observer identity encoding")
	}
	for i := range result {
		result[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	if result[5] <= 1 || result[5] > 1<<31-1 || result[7] == 0 {
		return [8]uint32{}, errors.New("invalid observer process identity")
	}
	return result, nil
}

func verifyObserver(k kernel, fd int, expected [8]uint32) error {
	if err := k.available(); err != nil {
		return err
	}
	self, err := k.self()
	if err != nil {
		return err
	}
	peer, err := k.peer(fd)
	if err != nil {
		return err
	}
	if peer != expected || peer[1] != self.audit[1] {
		return errors.New("control connection does not match trusted observer")
	}
	return nil
}

// Observer 保存同一次启动使用的宿主内核身份，不是执行授权。
type Observer struct {
	Identity string
	BootID   string
	UID      uint32
}

func ObserveSelf() (Observer, error) {
	identity, err := ObserverIdentity()
	if err != nil {
		return Observer{}, err
	}
	token, err := decodeIdentity(identity)
	if err != nil {
		return Observer{}, err
	}
	boot, err := CurrentBootID()
	if err != nil {
		return Observer{}, err
	}
	return Observer{Identity: identity, BootID: boot, UID: token[1]}, nil
}

// CurrentBootID 只接受内核启动 UUID，不能用时间或 PID 推断重启。
func CurrentBootID() (string, error) {
	boot, err := newKernel().bootID()
	if err != nil {
		return "", err
	}
	if !validBootID(boot) {
		return "", errors.New("invalid current boot identity")
	}
	return strings.ToLower(boot), nil
}
