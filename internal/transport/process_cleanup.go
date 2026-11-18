// INPUT: runtime 主进程退出后的 session 清理结果。
// OUTPUT: 可穿过 Wait/Close/启动错误包装的清理失败身份，以及排除退出码的清理结果。
// POS: 进程退出与资源回收完成的独立错误边界。
package transport

import (
	"errors"
	"fmt"
)

// ProcessCleanupError 表示后代 session 清理没有确认成功；主进程退出不能消除它。
// 成功清理 Unix session 也不证明 setsid 脱离的进程或全部平台后代已退出。
type ProcessCleanupError struct {
	SessionID int
	Err       error
}

// Error 将清理失败与主进程退出状态分开描述。
func (e *ProcessCleanupError) Error() string {
	return fmt.Sprintf("process: runtime session %d cleanup failed: %v", e.SessionID, e.Err)
}

// Unwrap 保留宿主信号或平台观察错误的原始身份。
func (e *ProcessCleanupError) Unwrap() error { return e.Err }

// processWaitResult 由持有主进程的 transport 在 Wait 处记录错误来源。
type processWaitResult struct {
	status  error
	cleanup error
}

type processDiagnostic struct {
	error
	cleanup error
}

func (d *processDiagnostic) Unwrap() error { return d.error }

func (w processWaitResult) diagnostic(stderr string, closeErr error) error {
	cleanup := errors.Join(w.cleanup, closeErr)
	err := normalizeExitErrorWithStderr(errors.Join(w.status, cleanup), stderr)
	if err == nil {
		return nil
	}
	return &processDiagnostic{error: err, cleanup: cleanup}
}

// CleanupError 只采用本 transport 已拆分的进程结果；自定义 transport 的错误保持原义。
// 这里刻意用类型断言而非 errors.As：结果一旦被外层包装，就无法确认包装层
// 没有附加清理失败，于是整体保守地视为清理错误，继续保留关闭栅栏。
func CleanupError(err error) error {
	if result, ok := err.(*processDiagnostic); ok {
		return result.cleanup
	}
	return err
}
