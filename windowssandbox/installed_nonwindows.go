//go:build !windows

// INPUT: 非Windows安装发现请求。
// OUTPUT: 明确不可用，不读取普通配置代替机器清单。
// POS: Windows机器安装发现的平台占位。
package windowssandbox

import (
	"context"
	"errors"
)

// LoadInstalled 仅支持Windows机器级安装。
func LoadInstalled(context.Context) (Installed, error) {
	return Installed{}, errors.New("Windows machine sandbox installation is unavailable on this platform")
}
