// INPUT: runtime 主进程退出后的 session 清理结果。
// OUTPUT: 可穿过 Wait/Close/启动错误包装的清理失败身份。
// POS: 进程退出与资源回收完成的独立错误边界。
package transport

import "fmt"

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
