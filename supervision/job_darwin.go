//go:build darwin && cgo

// INPUT: 随机唯一 job label 与可信宿主发布的 plist 路径。
// OUTPUT: 限时、限输出的系统 launchctl 观察/撤销结果。
// POS: job 不存在只证明无法再启动；集合退出仍单独观察。
package supervision

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type boundedOutput struct{ buffer bytes.Buffer }

func (b *boundedOutput) Write(data []byte) (int, error) {
	if b.buffer.Len()+len(data) > 1<<20 {
		return 0, errors.New("launchctl output exceeded limit")
	}
	return b.buffer.Write(data)
}
func (b *boundedOutput) String() string { return b.buffer.String() }

func launchctl(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/launchctl", args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	cmd.WaitDelay = time.Second
	var output boundedOutput
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	return output.String(), err
}
func domain(i Intent) string  { return fmt.Sprintf("gui/%d", i.OwnerUID) }
func service(i Intent) string { return domain(i) + "/" + i.JobLabel }
func jobPID(ctx context.Context, i Intent) (int, error) {
	output, err := launchctl(ctx, "print", service(i))
	if err != nil {
		return 0, fmt.Errorf("inspect bootstrap job: %w", err)
	}
	pid := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "\tpid = ") {
			value, err := strconv.Atoi(strings.TrimPrefix(line, "\tpid = "))
			if err != nil || value <= 1 || value > 1<<31-1 || pid != 0 {
				return 0, errors.New("ambiguous bootstrap job identity")
			}
			pid = value
		}
	}
	if pid == 0 {
		return 0, errors.New("bootstrap job has no live process")
	}
	return pid, nil
}
func removeJob(ctx context.Context, i Intent) error {
	_, err := launchctl(ctx, "bootout", service(i))
	if err == nil {
		return nil
	}
	output, lookupErr := launchctl(ctx, "print", service(i))
	var exit *exec.ExitError
	if errors.As(lookupErr, &exit) && exit.ExitCode() == 113 && strings.Contains(output, `Could not find service "`+i.JobLabel+`"`) {
		return nil
	}
	return errors.Join(fmt.Errorf("revoke bootstrap job: %w", err), lookupErr)
}
func jobDefinition(i Intent, helper, socket, observer string) []byte {
	quote := func(s string) string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
	return []byte(fmt.Sprintf(`<?xml version="1.0"?><plist version="1.0"><dict><key>Label</key><string>%s</string><key>ProgramArguments</key><array><string>%s</string><string>%s</string><string>%s</string></array><key>RunAtLoad</key><true/><key>KeepAlive</key><false/><key>AbandonProcessGroup</key><false/></dict></plist>`, quote(i.JobLabel), quote(helper), quote(socket), quote(observer)))
}
