// INPUT: 可信产品固定的机器映像、命令和持久授权适配器。
// OUTPUT: 与SDK实现独立的监督合同；准备/放行/清理分别持久确认。
// POS: Windows机器沙箱公开宿主边界，不接受模型直接提供授权投影。
package windowssandbox

import (
	"context"
	"encoding/json"
	"io"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
)

// Command 来自Bridge最终解析的runtime或probe命令，不经过额外shell解释。
type Command struct {
	Program     string
	Directory   string
	Arguments   []string
	Environment []string
}

// Intent 在启动机器Host之前持久保存，不把命令正文或环境秘密写入启动登记。
type Intent struct {
	Version       int
	LaunchID      string
	HelperSHA256  string
	CommandDigest [32]byte
}

// Prepared 绑定原产品启动意图与SDK生成的唯一执行、准备和授权清单。
type Prepared struct {
	LaunchID       string
	ExecutionID    string
	PrepareDigest  [32]byte
	ManifestDigest [32]byte
}

// Outcome 只有Cleaned为真才有完整SDK清理回执及机器Host实际退出证明。
// 非Cleaned保留责任；Prepared可在准备失败时仅含LaunchID。
type Outcome struct {
	Prepared
	ExitCode uint32
	Cleaned  bool
	Reason   string
}

// Host 由可信产品实现，不执行模型审批，也不自动重放请求。
// Reserve必须先持久记录Intent，再返回SDK严格schema的授权options正文；
// 正文必须绑定收到的精确Command和产品当前策略，不含凭据或旧执行身份。
// RecordPrepared持久记录实际双摘要，ClaimStart必须原子且只能消费一次。
// Finish独立持久记录cleaned或unknown；失败不能被helper退出掩盖。
type Host interface {
	Reserve(context.Context, Intent, Command) (json.RawMessage, error)
	RecordPrepared(context.Context, Intent, Prepared) error
	ClaimStart(context.Context, Intent, Prepared) error
	Finish(context.Context, Intent, Outcome) error
}

// Config 不允许用工作区中的任意nxs替代已安装的机器Host映像。
type Config struct {
	HelperPath   string
	HelperSHA256 string
	Host         Host
}

// Streams 由消费者并发排空，EOF或单独Wait退出码均不能替代清理结果。
type Streams struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

// Factory 每个正式执行及CLI probe申请独立授权登记，不复用未知执行。
type Factory func(context.Context, supervision.Purpose) (Config, error)
