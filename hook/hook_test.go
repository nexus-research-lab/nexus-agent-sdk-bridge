package hook

import (
	"reflect"
	"testing"
)

func TestOutputToMapDoesNotSerializeOnApplied(t *testing.T) {
	output := Output{
		SystemMessage: "continue",
		OnApplied:     func(AppliedAck) {},
	}
	if got, want := output.ToMap(), map[string]any{"systemMessage": "continue"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Output.ToMap() = %#v, want %#v", got, want)
	}
}
