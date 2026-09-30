// INPUT: 可信宿主传入的控制 socket 路径与固定宿主身份；任务只经认证连接传入。
// OUTPUT: 已登记 runtime 的原地 exec；失败只输出固定错误，不记录启动正文或凭据。
// POS: 随 Bridge 源码构建的 macOS 引导入口，尚未加入 Nexus 发布包。
package main

import (
	"fmt"
	"os"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/internal/processbootstrap"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "runtime bootstrap requires control socket and observer identity")
		os.Exit(2)
	}
	if err := processbootstrap.Run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "runtime bootstrap admission failed")
		os.Exit(1)
	}
}
