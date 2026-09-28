// INPUT: 可信宿主监督配置及冲突 transport/身份输入。
// OUTPUT: 配置保留到 process transport，冲突在连接前失败。
// POS: 公开选项回归，不运行内核或启动 helper。
package client

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
)

func TestProcessSupervisionOptions(t *testing.T) {
	factory := supervision.Factory(func(context.Context, supervision.Purpose) (supervision.Config, error) {
		return supervision.Config{}, nil
	})
	base := Options{CLIPath: "/bin/sh", ProcessSupervision: factory}
	normalized, err := base.normalized()
	if err != nil {
		t.Fatal(err)
	}
	if normalized.processConfig().Supervision == nil {
		t.Fatal("supervision lost during resolution")
	}
	for _, mutate := range []func(*Options){
		func(o *Options) { o.User = "someone" },
		func(o *Options) { o.DirectConnect = &DirectConnectOptions{URL: "http://localhost"} },
		func(o *Options) { o.Callbacks.ProcessSignalHandler = func(int, ProcessSignal) error { return nil } },
		func(o *Options) { o.Transport = fakeTransport{} },
	} {
		o := base
		mutate(&o)
		if _, err := o.normalized(); err == nil {
			t.Fatal("incompatible option accepted")
		}
	}
}
