package transport

import (
	"errors"
	"os"
	"testing"
)

func TestIsTransportWriteFailure(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "closed", err: os.ErrClosed, want: true},
		{name: "stdin", err: errors.New("stdin unavailable"), want: true},
		{name: "websocket", err: errors.New("websocket unavailable"), want: true},
		{name: "broken pipe", err: errors.New("write: broken pipe"), want: true},
		{name: "unrelated", err: errors.New("permission denied"), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTransportWriteFailure(tc.err); got != tc.want {
				t.Fatalf("IsTransportWriteFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestChannelClosed(t *testing.T) {
	open := make(chan struct{})
	if ChannelClosed(open) {
		t.Fatal("ChannelClosed(open) = true, want false")
	}
	close(open)
	if !ChannelClosed(open) {
		t.Fatal("ChannelClosed(closed) = false, want true")
	}
	if ChannelClosed(nil) {
		t.Fatal("ChannelClosed(nil) = true, want false")
	}
}
