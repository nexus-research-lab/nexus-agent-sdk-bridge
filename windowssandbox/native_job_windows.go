//go:build windows

// INPUT: 已Reserve的固定helper参数及当前产品进程。
// OUTPUT: 创建即归入匿名kill-on-close Job的悬挂helper。
// POS: 故障兜底不等于ACL已恢复；正常释放前必须真实查询Job为空。
package windowssandbox

import (
	"errors"
	"golang.org/x/sys/windows"
	"runtime"
	"unsafe"
)

func (m *nativeMachine) createHelper(application, command, directory *uint16, environment []uint16) error {
	var err error
	m.job, err = windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = 0x2000
	if _, err = windows.SetInformationJobObject(m.job, 9, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	var size uintptr
	nativeKernel.NewProc("InitializeProcThreadAttributeList").Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 || size > 65536 {
		return errors.New("invalid helper attribute size")
	}
	storage := make([]uint64, (size+7)/8)
	ptr := uintptr(unsafe.Pointer(&storage[0]))
	ok, _, callErr := nativeKernel.NewProc("InitializeProcThreadAttributeList").Call(ptr, 1, 0, uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return callErr
	}
	defer nativeKernel.NewProc("DeleteProcThreadAttributeList").Call(ptr)
	ok, _, callErr = nativeKernel.NewProc("UpdateProcThreadAttribute").Call(ptr, 0, 0x2000d, uintptr(unsafe.Pointer(&m.job)), unsafe.Sizeof(m.job), 0, 0)
	if ok == 0 {
		return callErr
	}
	type startupEx struct {
		Info       windows.StartupInfo
		Attributes uintptr
	}
	startup := startupEx{Attributes: ptr}
	startup.Info.Cb = uint32(unsafe.Sizeof(startup))
	err = windows.CreateProcess(application, command, nil, nil, false, 0x4|0x400|0x08000000|0x80000, &environment[0], directory, &startup.Info, &m.process)
	runtime.KeepAlive(storage)
	runtime.KeepAlive(environment)
	runtime.KeepAlive(m)
	if err != nil {
		return err
	}
	var member uint32
	ok, _, callErr = nativeKernel.NewProc("IsProcessInJob").Call(uintptr(m.process.Process), uintptr(m.job), uintptr(unsafe.Pointer(&member)))
	if ok == 0 || member == 0 {
		return errors.Join(errors.New("helper was not created inside owned Job"), callErr)
	}
	return nil
}
func (m *nativeMachine) verifyJobEmpty() error {
	if m.job == 0 {
		return nil
	}
	var accounting struct {
		User, Kernel, PeriodUser, PeriodKernel int64
		Faults, Total, Active, Terminated      uint32
	}
	ok, _, err := nativeKernel.NewProc("QueryInformationJobObject").Call(uintptr(m.job), 1, uintptr(unsafe.Pointer(&accounting)), unsafe.Sizeof(accounting), 0)
	if ok == 0 {
		return err
	}
	if accounting.Active != 0 {
		return errors.New("machine helper Job still contains live processes")
	}
	return nil
}
