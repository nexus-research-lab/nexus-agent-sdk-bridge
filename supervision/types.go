// INPUT: 可信宿主配置、显式执行输入与持久阶段适配。
// OUTPUT: 类型化启动意图、exact 集合回收事实和标准管道。
// POS: 宿主生命周期 API，不接受模型直接调用或任意恢复登记。
package supervision

import (
	"context"
	"os"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processbootstrap"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processscope"
)

// ErrUnavailable 表示当前平台缺少精确监督能力，调用方不得据此降级执行。
var ErrUnavailable = processscope.ErrUnavailable

type Command = processbootstrap.Launch
type Registration = processscope.Registration
type Exit = processscope.RootExit

// Intent 不包含任务正文、环境或命令参数；ID 必须绑定宿主自己的会话代次。
type Intent struct {
	Version      int
	ID           string
	BootID       string
	OwnerUID     uint32
	JobLabel     string
	HelperSHA256 string
}

type Paths struct {
	JobFile string
	Socket  string
}
type Evidence struct {
	Registration   Registration
	Reason         string
	ObservedBootID string
}

// Host 的所有输入来自可信调用方；实现必须将路径放在任务不可写的宿主目录。
// Reserve 先持久保存意图再返回私有路径；Publish 以固定目录句柄持久发布 job。
// Finish(nil) 只能撤销未登记意图；非 nil 必须核对原登记，不能顺带清除旧代次。
// Finish 还负责清除自身 job/socket 文件；结果未知时保留原记录，禁止自动重放。
type Host interface {
	Reserve(context.Context, Intent) (Paths, error)
	Publish(context.Context, Intent, []byte) error
	Register(context.Context, Intent, Registration) error
	ClaimRelease(context.Context, Intent) error
	Finish(context.Context, Intent, *Evidence) error
}

type Config struct {
	HelperPath   string
	HelperSHA256 string
	Host         Host
}

// Streams 的读端由消费者负责在读完后关闭；Start 失败会自行关闭已分配管道。
type Streams struct {
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}
