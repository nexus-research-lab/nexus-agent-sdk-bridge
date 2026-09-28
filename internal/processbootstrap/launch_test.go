// INPUT: 有效及畸形启动输入。
// OUTPUT: 拒绝含糊的环境、路径、版本和超限/尾随正文。
// POS: 引导协议校验测试；不执行命令。
package processbootstrap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLaunchRejectsMalformedInputs(t *testing.T) {
	valid := Launch{Version: 1, Command: "/bin/sh", Directory: "/tmp", Env: []string{"A=1"}}
	for _, name := range []string{"valid", "version", "command", "directory", "nul", "duplicate_env", "empty_key", "no_equals"} {
		t.Run(name, func(t *testing.T) {
			l := valid
			switch name {
			case "version":
				l.Version = 2
			case "command":
				l.Command = "sh"
			case "directory":
				l.Directory = "."
			case "nul":
				l.Args = []string{"x\x00y"}
			case "duplicate_env":
				l.Env = []string{"A=1", "A=2"}
			case "empty_key":
				l.Env = []string{"=x"}
			case "no_equals":
				l.Env = []string{"A"}
			}
			data, _ := json.Marshal(l)
			_, err := decodeLaunch(data)
			if (err == nil) != (name == "valid") {
				t.Fatalf("decode error=%v", err)
			}
		})
	}
	data, _ := json.Marshal(valid)
	for _, body := range []string{string(data) + " {}", `{"unknown":true}`, strings.Repeat(" ", maxLaunchBytes+1)} {
		if _, err := decodeLaunch([]byte(body)); err == nil {
			t.Fatal("invalid body accepted")
		}
	}
}
