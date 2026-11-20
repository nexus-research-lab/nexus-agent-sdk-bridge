package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func decodeToolResultBlock(t testing.TB, content any) ToolResultBlock {
	t.Helper()
	block := map[string]any{"type": "tool_result", "tool_use_id": "t1"}
	if content != nil {
		block["content"] = content
	}
	message, err := DecodeMessage(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": []any{block}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := AsToolResultBlock(message.User.Message.Content[0])
	if !ok {
		t.Fatalf("block = %#v, want tool_result", message.User.Message.Content[0])
	}
	return result
}

// TestToolResultBlockContentAccessors 对照解码值访问器与原先预编码 JSON 的读取语义。
func TestToolResultBlockContentAccessors(t *testing.T) {
	cases := map[string]struct {
		content    any
		wantString string
		wantOK     bool
	}{
		"string":  {"<b>\"quoted\"</b>", "<b>\"quoted\"</b>", true},
		"empty":   {"", "", true},
		"array":   {[]any{map[string]any{"type": "text", "text": "hi"}}, "", false},
		"object":  {map[string]any{"a": 1.0}, "", false},
		"missing": {nil, "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			block := decodeToolResultBlock(t, tc.content)
			if !reflect.DeepEqual(block.Content, tc.content) {
				t.Fatalf("Content = %#v, want decoded %#v", block.Content, tc.content)
			}
			assertToolResultReads(t, block, tc.content, tc.wantString, tc.wantOK)
		})
	}
}

// TestToolResultBlockAcceptsRawMessageContent 保持手工构造块时 json.RawMessage 的读取语义。
func TestToolResultBlockAcceptsRawMessageContent(t *testing.T) {
	assertToolResultReads(t, ToolResultBlock{Content: json.RawMessage(`"ok"`)}, "ok", "ok", true)
	assertToolResultReads(t, ToolResultBlock{Content: json.RawMessage(`[1,2]`)}, []any{1.0, 2.0}, "", false)
	assertToolResultReads(t, ToolResultBlock{Content: json.RawMessage(nil)}, nil, "", false)
}

func assertToolResultReads(t *testing.T, block ToolResultBlock, decoded any, wantString string, wantOK bool) {
	t.Helper()
	var wantJSON json.RawMessage
	if decoded != nil {
		wantJSON, _ = json.Marshal(decoded)
	}
	if got := block.ContentJSON(); string(got) != string(wantJSON) {
		t.Fatalf("ContentJSON = %s, want %s", got, wantJSON)
	}
	if got, ok := block.ContentString(); got != wantString || ok != wantOK {
		t.Fatalf("ContentString = %q, %t; want %q, %t", got, ok, wantString, wantOK)
	}
	var got any
	if err := block.DecodeContent(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, decoded) {
		t.Fatalf("DecodeContent = %#v, want %#v", got, decoded)
	}
}

// BenchmarkDecodeLargeToolResult 测量解码一条 256 KiB 文本工具结果消息。
func BenchmarkDecodeLargeToolResult(b *testing.B) {
	text := strings.Repeat("line \"quoted\" <tag>\n", 256<<10/22)
	payload := map[string]any{
		"type": "user",
		"message": map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": text},
		}},
	}
	for b.Loop() {
		if _, err := DecodeMessage(payload); err != nil {
			b.Fatal(err)
		}
	}
}
