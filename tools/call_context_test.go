package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestSDKMCPCallContextPreservesWireIdentity(t *testing.T) {
	received := make(chan Context, 32)
	tool := New("send", "test", nil, func(_ context.Context, input map[string]any, call *Context) (Result, error) {
		received <- *call
		return Text("ok"), nil
	})
	server := CreateSDKMCPServer(SDKMCPServerOptions{Name: "test", Tools: []Tool{tool}})
	call := func(ctx context.Context, meta map[string]any) {
		t.Helper()
		response, err := server.HandleMessage(ctx, map[string]any{"id": "call-tool", "method": "tools/call", "params": map[string]any{"name": "send", "_meta": meta, "arguments": map[string]any{"ToolUseID": "forged", "_meta": map[string]any{"claudecode/toolUseId": "forged"}}}})
		if err != nil || response["error"] != nil {
			t.Errorf("call failed: %v %v", response, err)
		}
	}
	call(context.Background(), map[string]any{"claudecode/toolUseId": "real-first"})
	if got := (<-received).ToolUseID; got != "real-first" {
		t.Fatalf("identity=%q", got)
	}
	call(context.Background(), map[string]any{"claudecode/toolUseId": "real-second"})
	if got := (<-received).ToolUseID; got != "real-second" {
		t.Fatalf("same body collapsed: %q", got)
	}
	parent := context.WithValue(context.Background(), callContextKey{}, Context{ToolUseID: "parent"})
	call(parent, nil)
	if got := (<-received).ToolUseID; got != "" {
		t.Fatalf("missing metadata inherited identity: %q", got)
	}
	call(parent, map[string]any{"claudecode/toolUseId": 42})
	if got := (<-received).ToolUseID; got != "" {
		t.Fatalf("invalid metadata became identity: %q", got)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			call(context.Background(), map[string]any{"claudecode/toolUseId": fmt.Sprintf("parallel-%d", i)})
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		got := <-received
		if seen[got.ToolUseID] {
			t.Fatalf("cross-call identity reuse: %q", got.ToolUseID)
		}
		seen[got.ToolUseID] = true
	}
	for i := 0; i < 20; i++ {
		if !seen[fmt.Sprintf("parallel-%d", i)] {
			t.Fatal("missing parallel identity")
		}
	}
}
