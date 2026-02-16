package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds the hctl configuration.
type Config struct {
	// RepoPath is the path to the gitops repository.
	RepoPath string `yaml:"repoPath"`
	// DefaultCluster is the default vCluster target for deploy commands.
	DefaultCluster string `yaml:"defaultCluster,omitempty"`
	// GitMode controls git behavior: "auto", "generate", or "prompt".
	GitMode string `yaml:"gitMode"`
	// ArgocdURL is the ArgoCD server URL.
	ArgocdURL string `yaml:"argocdURL"`
	// Interactive controls whether TUI wizards are used.
	Interactive bool `yaml:"interactive"`
	// KubeContext is the kubectl context to use.
	KubeContext string `yaml:"kubeContext,omitempty"`
	// Platform holds platform-specific settings.
	Platform PlatformConfig `yaml:"platform"`
}

// PlatformConfig holds settings specific to the homelab platform.
type PlatformConfig struct {
	// Domain is the base domain for the platform.
	Domain string `yaml:"domain"`
	// ClusterSubnet is the cluster's subnet CIDR.
	ClusterSubnet string `yaml:"clusterSubnet"`
	// MetalLBPool is the MetalLB IP range.
	MetalLBPool string `yaml:"metalLBPool"`
	// PlatformNamespace is the namespace for Kratix resource requests.
	PlatformNamespace string `yaml:"platformNamespace"`
	// StateRepo is the kratix-platform-state repository URL.
	StateRepo string `yaml:"stateRepo,omitempty"`
}

var (
	current *Config
	mu      sync.RWMutex
)

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		GitMode:     "prompt",
		ArgocdURL:   "https://argocd.cluster.integratn.tech",
		Interactive: true,
		Platform: PlatformConfig{
			Domain:            "cluster.integratn.tech",
			ClusterSubnet:     "10.0.4.0/24",
			MetalLBPool:       "10.0.4.200-253",
			PlatformNamespace: "platform-requests",
		},
	}
}

// ConfigDir returns the XDG config directory for hctl.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "hctl")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "hctl")
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// Load reads the config from a file path. If path is empty, uses the default location.
func Load(path string) (*Config, error) {
	if path == "" {
		path = ConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath(), data, 0o644)
}

// Set stores the active configuration.
func Set(cfg *Config) {
	mu.Lock()
	defer mu.Unlock()
	current = cfg
}

// Get returns the active configuration.
func Get() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return Default()
	}
	return current
}
