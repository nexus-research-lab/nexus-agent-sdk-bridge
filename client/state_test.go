package client

import (
	"context"
	"errors"
	"testing"
)

func TestPendingControlRequestsResolveAndDelete(t *testing.T) {
	pending := newPendingControlRequests()
	requestID := pending.nextID()
	waiter := pending.register(requestID)

	if !pending.resolve(requestID, controlWaitResult{Response: map[string]any{"ok": true}}) {
		t.Fatal("resolve() = false, want true")
	}
	result := <-waiter
	if result.Err != nil || result.Response["ok"] != true {
		t.Fatalf("result = %#v, want success payload", result)
	}
	if pending.resolve(requestID, controlWaitResult{}) {
		t.Fatal("resolve(deleted request) = true, want false")
	}

	deletedID := pending.nextID()
	pending.register(deletedID)
	pending.delete(deletedID)
	if pending.resolve(deletedID, controlWaitResult{}) {
		t.Fatal("resolve(after delete) = true, want false")
	}
}

func TestPendingControlRequestsRejectAll(t *testing.T) {
	pending := newPendingControlRequests()
	first := pending.register("first")
	second := pending.register("second")
	expectedErr := errors.New("transport closed")

	pending.rejectAll(expectedErr)
	for name, waiter := range map[string]<-chan controlWaitResult{"first": first, "second": second} {
		result := <-waiter
		if !errors.Is(result.Err, expectedErr) {
			t.Fatalf("%s result err = %v, want %v", name, result.Err, expectedErr)
		}
	}
	if pending.resolve("first", controlWaitResult{}) {
		t.Fatal("resolve(after rejectAll) = true, want false")
	}
}

func TestInflightControlRequestsCancelAndReset(t *testing.T) {
	inflight := newInflightControlRequests()
	ctx, cancel := context.WithCancel(context.Background())
	inflight.add("one", cancel)

	if !inflight.cancel("one") {
		t.Fatal("cancel() = false, want true")
	}
	if ctx.Err() == nil {
		t.Fatal("context was not canceled")
	}
	if inflight.cancel("one") {
		t.Fatal("cancel(second call) = true, want false")
	}

	_, secondCancel := context.WithCancel(context.Background())
	inflight.add("two", secondCancel)
	inflight.reset()
	if inflight.cancel("two") {
		t.Fatal("cancel(after reset) = true, want false")
	}
}
