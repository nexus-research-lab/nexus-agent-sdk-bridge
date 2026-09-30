//go:build windows

// INPUT: 当前安装Host账号与真实helper进程对象。
// OUTPUT: 四条独立本机管道，每次传输核验同一helper身份。
// POS: 同步API内部采用可取消overlapped IO，不用PID终止或未知重连。
package windowssandbox

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"golang.org/x/sys/windows"
	"io"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

type nativePipe struct {
	mu          sync.Mutex
	writeMu     sync.Mutex
	eventMu     sync.Mutex
	eventFailed bool
	ctx         context.Context
	cancel      context.CancelFunc
	handle      windows.Handle
	name        string
	owner       *nativeMachine
	events      []windows.Handle
	pending     []byte
	closed      bool
	failure     error
	stdin       bool
}

func newNativePipe(owner *nativeMachine) (*nativePipe, error) {
	ctx, cancel := context.WithCancel(owner.ctx)
	p := &nativePipe{owner: owner, ctx: ctx, cancel: cancel}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return p, err
	}
	p.name = `\\.\pipe\nexus-machine-host-` + hex.EncodeToString(nonce[:])
	sd, err := windows.SecurityDescriptorFromString("O:" + owner.hostSID + "D:P(A;;RC;;;OW)(A;;0x12019b;;;" + owner.hostSID + ")")
	if err != nil {
		return p, err
	}
	name, _ := windows.UTF16PtrFromString(p.name)
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	p.handle, err = windows.CreateNamedPipe(name, 0x40080003, 8, 1, 65536, 65536, 5000, &sa)
	if err != nil {
		p.handle = 0
	}
	return p, err
}

// verifyPeer 对实际创建时间和账号前后核验，排空时允许原helper已退出但不接受不同内核peerPID。
func (p *nativePipe) verifyPeer(live bool) error {
	if p.owner.process.Process == 0 {
		return errors.New("missing helper identity")
	}
	var pid uint32
	ok, _, err := nativeKernel.NewProc("GetNamedPipeClientProcessId").Call(uintptr(p.handle), uintptr(unsafe.Pointer(&pid)))
	if ok == 0 {
		return err
	}
	if pid != p.owner.process.ProcessId {
		return errors.New("pipe peer differs from launched helper")
	}
	created, err := nativeCreated(p.owner.process.Process)
	if err != nil || created != p.owner.created {
		return errors.New("helper creation identity changed")
	}
	if live {
		state, err := windows.WaitForSingleObject(p.owner.process.Process, 0)
		if err != nil || state != windows.WAIT_TIMEOUT {
			return errors.New("helper is not alive")
		}
	}
	if err = p.owner.verifyAccount(p.owner.process.Process); err != nil {
		return err
	}
	if live {
		state, err := windows.WaitForSingleObject(p.owner.process.Process, 0)
		if err != nil || state != windows.WAIT_TIMEOUT {
			return errors.New("helper exited during verification")
		}
	}
	return nil
}

// waitIO 取消后仍等待内核完成，保证OVERLAPPED和Go缓冲区未提前释放。
func (p *nativePipe) waitIO(ctx context.Context, overlap *windows.Overlapped) (uint32, error) {
	for {
		state, err := windows.WaitForSingleObject(overlap.HEvent, 20)
		if err != nil {
			cancelErr := windows.CancelIoEx(p.handle, overlap)
			var count uint32
			waitErr := windows.GetOverlappedResult(p.handle, overlap, &count, true)
			return count, errors.Join(err, cancelErr, waitErr)
		}
		if state == windows.WAIT_OBJECT_0 {
			var count uint32
			err = windows.GetOverlappedResult(p.handle, overlap, &count, false)
			return count, err
		}
		if err = ctx.Err(); err != nil {
			cancelErr := windows.CancelIoEx(p.handle, overlap)
			if cancelErr == windows.ERROR_NOT_FOUND {
				cancelErr = nil
			}
			var count uint32
			waitErr := windows.GetOverlappedResult(p.handle, overlap, &count, true)
			return count, errors.Join(err, cancelErr, waitErr)
		}
	}
}
func (p *nativePipe) event() (*windows.Overlapped, error) {
	p.eventMu.Lock()
	defer p.eventMu.Unlock()
	if p.eventFailed {
		return nil, errors.New("pipe event cleanup remains unknown")
	}
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return nil, err
	}
	p.events = append(p.events, event)
	return &windows.Overlapped{HEvent: event}, nil
}
func (p *nativePipe) releaseEvent(event windows.Handle) error {
	p.eventMu.Lock()
	defer p.eventMu.Unlock()
	if err := windows.CloseHandle(event); err != nil {
		p.eventFailed = true
		return err
	}
	for i, h := range p.events {
		if h == event {
			p.events = append(p.events[:i], p.events[i+1:]...)
			break
		}
	}
	return nil
}
func (p *nativePipe) connect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	overlap, err := p.event()
	if err != nil {
		return err
	}
	err = windows.ConnectNamedPipe(p.handle, overlap)
	switch err {
	case windows.ERROR_PIPE_CONNECTED:
		err = nil
	case windows.ERROR_IO_PENDING:
		_, err = p.waitIO(ctx, overlap)
	}
	err = errors.Join(err, p.releaseEvent(overlap.HEvent))
	if err != nil {
		return err
	}
	return p.verifyPeer(true)
}
func (p *nativePipe) transfer(ctx context.Context, data []byte, write, live bool) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := p.verifyPeer(live); err != nil {
		return 0, err
	}
	overlap, err := p.event()
	if err != nil {
		return 0, err
	}
	var count uint32
	if write {
		err = windows.WriteFile(p.handle, data, &count, overlap)
	} else {
		err = windows.ReadFile(p.handle, data, &count, overlap)
	}
	if err == windows.ERROR_IO_PENDING {
		count, err = p.waitIO(ctx, overlap)
	}
	runtime.KeepAlive(data)
	err = errors.Join(err, p.releaseEvent(overlap.HEvent))
	if err == nil {
		err = p.verifyPeer(live)
	}
	if !write && errors.Is(err, windows.ERROR_BROKEN_PIPE) {
		return int(count), io.EOF
	}
	return int(count), err
}
func (p *nativePipe) full(ctx context.Context, data []byte, write bool) error {
	for len(data) > 0 {
		n, err := p.transfer(ctx, data, write, write)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrUnexpectedEOF
		}
		data = data[n:]
	}
	return nil
}
func (p *nativePipe) writeFrame(ctx context.Context, body []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if len(body) > maxControlFrame {
		return errors.New("machine frame exceeds bound")
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(body)))
	if err := p.full(ctx, header[:], true); err != nil {
		return err
	}
	return p.full(ctx, body, true)
}
func (p *nativePipe) readFrame(ctx context.Context) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var header [4]byte
	if err := p.full(ctx, header[:], false); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size == 0 || size > maxControlFrame {
		return nil, errors.New("machine control frame exceeds bound")
	}
	body := make([]byte, int(size))
	err := p.full(ctx, body, false)
	return body, err
}
func (p *nativePipe) Read(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	if len(data) == 0 {
		return 0, nil
	}
	return p.transfer(p.ctx, data[:min(len(data), 65536)], false, false)
}
func (p *nativePipe) Write(data []byte) (int, error) {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if p.closed || !p.stdin {
		return 0, io.ErrClosedPipe
	}
	bounded, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()
	written := 0
	for len(data) > 0 {
		size := min(len(data), 65536)
		var header [4]byte
		binary.LittleEndian.PutUint32(header[:], uint32(size))
		if err := p.full(bounded, header[:], true); err != nil {
			p.failure = err
			p.closed = true
			p.cancel()
			return written, err
		}
		if err := p.full(bounded, data[:size], true); err != nil {
			p.failure = err
			p.closed = true
			p.cancel()
			return written, err
		}
		written += size
		data = data[size:]
	}
	return written, nil
}
func (p *nativePipe) Close() error {
	if p.stdin {
		p.writeMu.Lock()
		defer p.writeMu.Unlock()
	} else {
		p.cancel()
		p.mu.Lock()
		defer p.mu.Unlock()
	}
	if p.closed {
		return p.failure
	}
	if p.stdin {
		var end [4]byte
		bounded, cancel := context.WithTimeout(p.ctx, 10*time.Second)
		defer cancel()
		if err := p.full(bounded, end[:], true); err != nil {
			p.failure = err
			p.closed = true
			p.cancel()
			return err
		}
	}
	p.closed = true
	return nil
}
func (p *nativePipe) release() error {
	p.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	for len(p.events) > 0 {
		if err := p.releaseEvent(p.events[0]); err != nil {
			return err
		}
	}
	if p.handle != 0 {
		if err := windows.CloseHandle(p.handle); err != nil {
			return err
		}
		p.handle = 0
	}
	return nil
}
