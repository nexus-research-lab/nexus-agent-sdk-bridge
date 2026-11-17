// INPUT: runtime 主进程退出后的 session 清理结果。
// OUTPUT: 可穿过 Wait/Close/启动错误包装的清理失败身份，以及排除退出码的清理结果。
// POS: 进程退出与资源回收完成的独立错误边界。
package transport

import (
	"errors"
	"fmt"
	"os/exec"
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

// CleanupError 只剔除已确认的进程退出状态；清理失败与未知错误继续阻止会话复用。
// 必须逐项处理 joined error，不能因其中存在非零退出码而吞掉其他清理失败。
func CleanupError(err error) error {
	if _, ok := err.(*ProcessCleanupError); ok {
		return err
	}
	switch value := err.(type) {
	case *exec.ExitError, *supervisedProcessExitError:
		return nil
	case interface{ Unwrap() []error }:
		var failures []error
		for _, cause := range value.Unwrap() {
			failures = append(failures, CleanupError(cause))
		}
		return errors.Join(failures...)
	case interface{ Unwrap() error }:
		if cause := value.Unwrap(); cause != nil && CleanupError(cause) == nil {
			return nil
		}
	}
	return err
}
