// INPUT: 已认证机器Host控制管道中的有界消息和原准备回执。
// OUTPUT: 精确schema校验的准备/清理结果，不能由进程退出代替完整清理证据。
// POS: Windows机器沙箱跨仓协议；不导入SDK内部实现，不执行任务或审批。
package windowssandbox

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

const maxControlFrame = 1 << 20

type controlFrame struct {
	Version        int             `json:"version"`
	Kind           string          `json:"kind"`
	LaunchID       string          `json:"launchID"`
	ExecutionID    string          `json:"executionID"`
	PrepareDigest  [32]byte        `json:"prepareDigest"`
	ManifestDigest [32]byte        `json:"manifestDigest"`
	Options        json.RawMessage `json:"options,omitempty"`
	ExitCode       uint32          `json:"exitCode"`
	Reason         string          `json:"reason"`
	Retained       bool            `json:"retained"`
}

// validCorrelation 只接受非零规范随机关联名，不把产品launch与SDK执行身份混用。
func validCorrelation(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 16 && hex.EncodeToString(decoded) == value && !bytes.Equal(decoded, make([]byte, 16))
}

// decodeControlReply 在typed解码前核验原字段拼写、完整digest长度及所有层级重复键。
func decodeControlReply(body []byte, launchID string) (controlFrame, error) {
	var frame controlFrame
	if len(body) == 0 || len(body) > maxControlFrame || !validCorrelation(launchID) {
		return frame, errors.New("invalid Windows machine control reply size or launch")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := uniqueControlValue(decoder, 0); err != nil {
		return frame, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return frame, errors.New("trailing Windows machine control reply")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || len(fields) != 9 {
		return frame, errors.New("incomplete Windows machine control reply")
	}
	for _, key := range []string{"version", "kind", "launchID", "executionID", "prepareDigest", "manifestDigest", "exitCode", "reason", "retained"} {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return frame, errors.New("missing or null Windows machine control field")
		}
	}
	for _, key := range []string{"prepareDigest", "manifestDigest"} {
		var digest []json.RawMessage
		if err := json.Unmarshal(fields[key], &digest); err != nil || len(digest) != 32 {
			return frame, errors.New("invalid Windows machine control digest length")
		}
		for _, item := range digest {
			var octet uint8
			if bytes.Equal(bytes.TrimSpace(item), []byte("null")) || json.Unmarshal(item, &octet) != nil {
				return frame, errors.New("invalid Windows machine control digest byte")
			}
		}
	}
	if err := json.Unmarshal(body, &frame); err != nil {
		return frame, errors.New("invalid Windows machine control reply values")
	}
	if frame.Version != 1 || frame.LaunchID != launchID || len(frame.Reason) > 2048 {
		return frame, errors.New("Windows machine control reply differs from launch")
	}
	switch frame.Kind {
	case "prepared":
		if !frame.bound() || frame.Retained || frame.Reason != "" || frame.ExitCode != 0 {
			return frame, errors.New("invalid Windows machine prepared receipt")
		}
	case "cleaned":
		if !frame.bound() || frame.Retained || frame.Reason != "" {
			return frame, errors.New("invalid Windows machine cleanup receipt")
		}
	case "unknown":
		if !frame.Retained || frame.Reason == "" {
			return frame, errors.New("invalid Windows machine unknown receipt")
		}
	default:
		return frame, errors.New("unexpected Windows machine control reply kind")
	}
	return frame, nil
}

// bound 检查已准备执行的完整关联；unknown在准备失败时可尚未获得执行双摘要。
func (f controlFrame) bound() bool {
	return validCorrelation(f.ExecutionID) && f.PrepareDigest != ([32]byte{}) && f.ManifestDigest != ([32]byte{})
}

// matchesPrepared 防止接受其他执行或其他授权清单的终态，调用方另检查具体阶段。
func (f controlFrame) matchesPrepared(prepared controlFrame) bool {
	return f.LaunchID == prepared.LaunchID && f.ExecutionID == prepared.ExecutionID && f.PrepareDigest == prepared.PrepareDigest && f.ManifestDigest == prepared.ManifestDigest
}

// continuationFrame 只生成引用原准备的start/cancel/cleanup，不允许重发授权投影。
func continuationFrame(kind string, prepared controlFrame) ([]byte, error) {
	if kind != "start" && kind != "cancel" && kind != "cleanup" || prepared.Kind != "prepared" || !prepared.bound() {
		return nil, errors.New("invalid Windows machine continuation")
	}
	return json.Marshal(controlFrame{Version: 1, Kind: kind, LaunchID: prepared.LaunchID, ExecutionID: prepared.ExecutionID,
		PrepareDigest: prepared.PrepareDigest, ManifestDigest: prepared.ManifestDigest})
}

// uniqueControlValue 保留数字精度，逐对象拒绝重复字段并限制嵌套深度。
func uniqueControlValue(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("Windows machine control JSON is too deeply nested")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	var closing json.Delim
	switch delimiter {
	case '{':
		closing = '}'
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate Windows machine control field")
			}
			seen[name] = true
			if err := uniqueControlValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		closing = ']'
		for decoder.More() {
			if err := uniqueControlValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected Windows machine control delimiter")
	}
	token, err = decoder.Token()
	if err != nil || token != closing {
		return errors.New("invalid Windows machine control JSON termination")
	}
	return nil
}
