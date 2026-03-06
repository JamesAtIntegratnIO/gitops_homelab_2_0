package config

import "context"

type cfgKey struct{}

// NewContext returns a copy of ctx carrying the given Config.
func NewContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, cfgKey{}, cfg)
}

// FromContext extracts the Config from ctx. If no Config is present it returns
// nil and false.
func FromContext(ctx context.Context) (*Config, bool) {
	cfg, ok := ctx.Value(cfgKey{}).(*Config)
	return cfg, ok && cfg != nil
}
