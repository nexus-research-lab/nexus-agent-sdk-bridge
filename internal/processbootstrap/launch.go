// INPUT: 宿主已经校验并允许执行的命令、环境和工作目录。
// OUTPUT: 单次 exec 的限长、无隐式环境继承的闭合输入。
// POS: 内部引导协议，不接收模型控制命令，不授予额外文件或网络权限。
package processbootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

const maxLaunchBytes = 1 << 20

// Launch 只能在宿主完成持久登记后交给已认证 helper；不得写入 plist 或日志。
type Launch struct {
	Version   int      `json:"version"`
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	Env       []string `json:"env"`
	Directory string   `json:"directory"`
}

func (l Launch) validate() error {
	if l.Version != 1 || !filepath.IsAbs(l.Command) || !filepath.IsAbs(l.Directory) {
		return errors.New("invalid bootstrap launch contract")
	}
	for _, value := range append([]string{l.Command, l.Directory}, l.Args...) {
		if strings.ContainsRune(value, 0) {
			return errors.New("bootstrap launch contains NUL")
		}
	}
	seen := make(map[string]bool)
	for _, value := range l.Env {
		key, _, ok := strings.Cut(value, "=")
		if !ok || key == "" || strings.ContainsRune(value, 0) || seen[key] {
			return errors.New("invalid bootstrap environment")
		}
		seen[key] = true
	}
	return nil
}

func decodeLaunch(data []byte) (Launch, error) {
	var l Launch
	if len(data) == 0 || len(data) > maxLaunchBytes {
		return l, errors.New("invalid bootstrap launch size")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&l); err != nil {
		return Launch{}, errors.New("invalid bootstrap launch payload")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Launch{}, errors.New("trailing bootstrap launch payload")
	}
	return l, l.validate()
}

// Validate 检查发送前的完整输入，失败不得创建启动意图。
func (l Launch) Validate() error {
	if err := l.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if len(data) > maxLaunchBytes {
		return errors.New("bootstrap launch is too large")
	}
	return nil
}
