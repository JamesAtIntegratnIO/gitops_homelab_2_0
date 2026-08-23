package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the agent's whole configuration, read from the environment so a
// chart can supply it and a Secret can supply the two values that matter.
//
// There is no default model provider on purpose. A component that installs
// cleanly and then quietly starts spending money against a vendor the operator
// did not choose is a bad default, however convenient.
type Config struct {
	Addr string

	// Git host.
	GitProvider string // github | gitea
	// GitAPIBase means different things per host, because the hosts do:
	// on GitHub it is the API root (.../api/v3 for Enterprise), on Gitea it
	// is the INSTANCE root and the client appends /api/v1 itself, because it
	// also needs that root to build a push remote.
	GitAPIBase  string
	GitOwner    string
	GitRepo     string
	GitRepoURL  string
	GitToken    string
	AuthorName  string
	AuthorEmail string
	// GitInsecureSkipTLSVerify allows a self-signed certificate on a
	// self-hosted host. Scoped to the git client; never process-wide.
	GitInsecureSkipTLSVerify bool

	// Model.
	LLMProvider        string // openai | anthropic
	LLMBaseURL         string
	LLMModel           string
	LLMKey             string
	LLMReasoningEffort string
	LLMTimeout         time.Duration

	// Identity. The agent signs its comments and commits with this, and it is
	// deliberately NOT the same thing as the account the token belongs to --
	// a reviewer should be able to tell a bot's comment from a colleague's at
	// a glance, and the token's owner is whoever minted it.
	Brand     string
	BrandMark string

	// Behaviour.
	CheckName   string
	MaxAttempts int
	GateWait    time.Duration
	GatePoll    time.Duration
	AllowPaths  []string
	DenyPaths   []string
	CloneRoot   string
}

func LoadConfig() (*Config, error) {
	c := &Config{
		Addr:                     env("AGENT_ADDR", ":8080"),
		Brand:                    env("AGENT_BRAND", "Bosun"),
		BrandMark:                os.Getenv("AGENT_BRAND_MARK"),
		GitProvider:              env("GIT_PROVIDER", "github"),
		GitInsecureSkipTLSVerify: os.Getenv("GIT_INSECURE_SKIP_TLS_VERIFY") == "true",
		GitAPIBase:               os.Getenv("GIT_API_BASE"),
		GitOwner:                 os.Getenv("GIT_OWNER"),
		GitRepo:                  os.Getenv("GIT_REPO"),
		GitRepoURL:               os.Getenv("GIT_REPO_URL"),
		GitToken:                 os.Getenv("GIT_TOKEN"),
		AuthorName:               env("GIT_AUTHOR_NAME", "bosun"),
		AuthorEmail:              env("GIT_AUTHOR_EMAIL", "bosun@users.noreply.github.com"),

		LLMProvider:        os.Getenv("LLM_PROVIDER"),
		LLMBaseURL:         os.Getenv("LLM_BASE_URL"),
		LLMModel:           os.Getenv("LLM_MODEL"),
		LLMKey:             os.Getenv("LLM_API_KEY"),
		LLMReasoningEffort: os.Getenv("LLM_REASONING_EFFORT"),

		CheckName: env("GATE_CHECK_NAME", "addons-gate"),
		CloneRoot: env("CLONE_ROOT", ""),
	}

	var err error
	if c.MaxAttempts, err = envInt("MAX_ATTEMPTS", 2); err != nil {
		return nil, err
	}
	if c.GateWait, err = envDur("GATE_WAIT", 10*time.Minute); err != nil {
		return nil, err
	}
	if c.GatePoll, err = envDur("GATE_POLL", 30*time.Second); err != nil {
		return nil, err
	}
	if c.LLMTimeout, err = envDur("LLM_TIMEOUT", 10*time.Minute); err != nil {
		return nil, err
	}
	c.AllowPaths = envList("ALLOW_PATHS")
	c.DenyPaths = envList("DENY_PATHS")

	return c, c.validate()
}

func (c *Config) validate() error {
	var missing []string
	need := map[string]string{
		"GIT_OWNER": c.GitOwner, "GIT_REPO": c.GitRepo,
		"GIT_REPO_URL": c.GitRepoURL, "GIT_TOKEN": c.GitToken,
		"LLM_PROVIDER": c.LLMProvider, "LLM_MODEL": c.LLMModel,
	}
	for k, v := range need {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	switch c.LLMProvider {
	case "openai":
		if c.LLMBaseURL == "" {
			return fmt.Errorf("LLM_BASE_URL is required for the openai provider (it is what makes a self-hosted model work)")
		}
	case "anthropic":
	default:
		return fmt.Errorf("unknown LLM_PROVIDER %q (openai or anthropic)", c.LLMProvider)
	}

	switch c.GitProvider {
	case "github", "gitea":
	default:
		return fmt.Errorf("GIT_PROVIDER %q is not implemented yet -- see docs/git-providers.md", c.GitProvider)
	}

	// An empty allowlist means the agent can write nothing. That is the safe
	// default, but running with it is almost certainly a misconfiguration, so
	// say so at startup rather than silently refusing every fix later.
	if len(c.AllowPaths) == 0 {
		return fmt.Errorf("ALLOW_PATHS is empty: the agent could never apply any fix")
	}
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) (int, error) {
	v := os.Getenv(k)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", k, err)
	}
	return n, nil
}

func envDur(k string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(k)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", k, err)
	}
	return d, nil
}

func envList(k string) []string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
