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

// Shared returns a lazily-initialized kube client using config.Get().KubeContext.
// Subsequent calls return the same client and error. Call ResetShared() in tests
// to clear the cached client.
func Shared() (*Client, error) {
	sharedOnce.Do(func() {
		cfg := config.Get()
		sharedClient, sharedErr = NewClient(cfg.KubeContext)
	})
	return sharedClient, sharedErr
}

// ResetShared clears the cached client so the next Shared() call re-initializes.
// Intended for testing only.
func ResetShared() {
	sharedOnce = sync.Once{}
	sharedClient = nil
	sharedErr = nil
}
