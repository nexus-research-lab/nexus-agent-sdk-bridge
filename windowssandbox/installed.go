// INPUT: 受保护机器安装清单。
// OUTPUT: 产品装配所需固定helper路径、摘要及包版本/架构。
// POS: 只读安装发现，不表示沙箱capability已通过验收。
package windowssandbox

// Installed 的摘要来自机器安装根，不接受任务环境或工作区覆盖。
type Installed struct {
	HelperPath     string
	HelperSHA256   string
	PackageVersion string
	Architecture   string
}
