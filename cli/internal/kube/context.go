package kube

import (
	"context"
	"sync"

	"github.com/jamesatintegratnio/hctl/internal/config"
)

// --- Context helpers for passing a kube client through context.Context ---

type kubeClientKey struct{}

// NewContext returns a copy of ctx carrying the kube client c.
func NewContext(ctx context.Context, c *Client) context.Context {
	return context.WithValue(ctx, kubeClientKey{}, c)
}

// FromContext extracts the kube client from ctx, if present.
func FromContext(ctx context.Context) (*Client, bool) {
	c, ok := ctx.Value(kubeClientKey{}).(*Client)
	return c, ok && c != nil
}

// --- Shared (lazy-singleton) client ---

var (
	sharedOnce   sync.Once
	sharedClient *Client
	sharedErr    error
)

// SharedWithConfig returns a lazily-initialized kube client using the supplied
// kubeContext. Subsequent calls return the same client and error regardless of
// the kubeContext value. Call ResetShared() in tests to clear the cached client.
func SharedWithConfig(kubeContext string) (*Client, error) {
	sharedOnce.Do(func() {
		sharedClient, sharedErr = NewClient(kubeContext)
	})
	return sharedClient, sharedErr
}

// Shared returns a lazily-initialized kube client using config.Get().KubeContext.
// It is a convenience wrapper intended for the cmd/ layer (composition root).
func Shared() (*Client, error) {
	return SharedWithConfig(config.Get().KubeContext)
}

// ResetShared clears the cached client so the next Shared() call re-initializes.
// Intended for testing only.
func ResetShared() {
	sharedOnce = sync.Once{}
	sharedClient = nil
	sharedErr = nil
}
