// INPUT: 已固定的宿主身份与连接内核身份。
// OUTPUT: 错误宿主、跨用户和非法编码全部拒绝。
// POS: 引导双向身份校验测试。
package processscope

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestVerifyObserverRequiresExactSameUserIdentity(t *testing.T) {
	for _, name := range []string{"valid", "version", "uid", "unavailable"} {
		t.Run(name, func(t *testing.T) {
			k := fixture()
			identity := k.root.audit
			switch name {
			case "version":
				identity[7]++
			case "uid":
				identity[1] = 502
				k.root.audit[1] = 502
			case "unavailable":
				k.availableErr = ErrUnavailable
			}
			err := verifyObserver(k, 10, identity)
			if (err == nil) != (name == "valid") {
				t.Fatalf("verify=%v", err)
			}
		})
	}
}
func TestObserverIdentityEncoding(t *testing.T) {
	var raw [32]byte
	k := fixture()
	for i, value := range k.root.audit {
		binary.BigEndian.PutUint32(raw[i*4:], value)
	}
	encoded := hex.EncodeToString(raw[:])
	identity, err := decodeIdentity(encoded)
	if err != nil || identity != k.root.audit {
		t.Fatalf("decode=%v", err)
	}
	for _, value := range []string{"", encoded[:62], encoded + "00", encoded[:63] + "z", hex.EncodeToString(make([]byte, 32))} {
		if _, err := decodeIdentity(value); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
	if err := VerifyObserver(nil, encoded); err == nil {
		t.Fatal("missing connection accepted")
	}
}
