package kube

import (
	"context"
	"sync"
	"testing"
)

func TestKubeNewContext_FromContext_RoundTrip(t *testing.T) {
	client := &Client{} // empty client, just testing context plumbing

	ctx := NewContext(context.Background(), client)

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext returned false")
	}
	if got != client {
		t.Error("FromContext returned different client")
	}
}

func TestKubeFromContext_EmptyContext(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Error("FromContext on empty context should return false")
	}
}

func TestKubeFromContext_NilClient(t *testing.T) {
	ctx := context.WithValue(context.Background(), kubeClientKey{}, (*Client)(nil))

	_, ok := FromContext(ctx)
	if ok {
		t.Error("FromContext with nil Client should return false")
	}
}

func TestResetShared_ClearsState(t *testing.T) {
	// First, set the shared state manually to simulate prior initialization
	sharedOnce = sync.Once{}
	sharedClient = &Client{}
	sharedErr = nil

	// After reset, the shared state should be cleared
	ResetShared()

	if sharedClient != nil {
		t.Error("sharedClient should be nil after ResetShared")
	}
	if sharedErr != nil {
		t.Error("sharedErr should be nil after ResetShared")
	}
}

func TestResetShared_AllowsReinitialization(t *testing.T) {
	// Reset to ensure clean state
	ResetShared()

	// Track if Do was called
	called := false
	sharedOnce.Do(func() {
		called = true
	})
	if !called {
		t.Error("expected Do to execute after ResetShared")
	}

	// Second Do should not call again
	calledAgain := false
	sharedOnce.Do(func() {
		calledAgain = true
	})
	if calledAgain {
		t.Error("second Do should not execute")
	}

	// After reset, Do should fire again
	ResetShared()
	calledThird := false
	sharedOnce.Do(func() {
		calledThird = true
	})
	if !calledThird {
		t.Error("Do should execute again after ResetShared")
	}
}
