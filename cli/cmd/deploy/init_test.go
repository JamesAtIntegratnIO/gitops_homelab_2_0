package deploy

import (
	"strings"
	"testing"
)

func TestGenerateScoreTemplate_Web(t *testing.T) {
	result := generateScoreTemplate("web", "my-app", "dev-cluster", "example.com")

	checks := []struct {
		name     string
		contains string
	}{
		{"has apiVersion", "apiVersion: score.dev/v1b1"},
		{"has workload name", "name: my-app"},
		{"has cluster annotation", `hctl.integratn.tech/cluster: "dev-cluster"`},
		{"has image placeholder", `image: "."`},
		{"has route resource", "type: route"},
		{"has hostname", "host: my-app.example.com"},
		{"has port 8080", "port: 8080"},
		{"has health probes", "/healthz"},
		{"has readiness probe", "/readyz"},
		{"has memory request", "memory:"},
		{"has cpu request", "cpu:"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(result, c.contains) {
				t.Errorf("web template missing %q", c.contains)
			}
		})
	}
}

func TestGenerateScoreTemplate_API(t *testing.T) {
	result := generateScoreTemplate("api", "my-api", "prod", "example.com")

	checks := []struct {
		name     string
		contains string
	}{
		{"has workload name", "name: my-api"},
		{"has cluster annotation", `hctl.integratn.tech/cluster: "prod"`},
		{"has postgres resource", "type: postgres"},
		{"has DB_HOST env", "DB_HOST:"},
		{"has DB_PORT env", "DB_PORT:"},
		{"has DB_NAME env", "DB_NAME:"},
		{"has DB_USER env", "DB_USER:"},
		{"has DB_PASS env", "DB_PASS:"},
		{"has hostname", "host: my-api.example.com"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(result, c.contains) {
				t.Errorf("api template missing %q", c.contains)
			}
		})
	}
}

func TestGenerateScoreTemplate_Worker(t *testing.T) {
	result := generateScoreTemplate("worker", "my-worker", "staging", "example.com")

	checks := []struct {
		name     string
		contains string
	}{
		{"has workload name", "name: my-worker"},
		{"has cluster annotation", `hctl.integratn.tech/cluster: "staging"`},
		{"has worker container", "worker:"},
		{"has health port", "port: 8080"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(result, c.contains) {
				t.Errorf("worker template missing %q", c.contains)
			}
		})
	}

	// Worker should NOT have a route resource
	if strings.Contains(result, "type: route") {
		t.Error("worker template should not have a route resource")
	}
}

func TestGenerateScoreTemplate_Cron(t *testing.T) {
	result := generateScoreTemplate("cron", "my-job", "dev", "example.com")

	checks := []struct {
		name     string
		contains string
	}{
		{"has workload name", "name: my-job"},
		{"has cluster annotation", `hctl.integratn.tech/cluster: "dev"`},
		{"has job container", "job:"},
		{"has command", "command:"},
		{"has empty resources", "resources: {}"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(result, c.contains) {
				t.Errorf("cron template missing %q", c.contains)
			}
		})
	}

	// Cron should NOT have service or route
	if strings.Contains(result, "type: route") {
		t.Error("cron template should not have a route resource")
	}
}

func TestGenerateScoreTemplate_DefaultIsWeb(t *testing.T) {
	web := generateScoreTemplate("web", "test", "c", "d.com")
	unknown := generateScoreTemplate("unknown-template", "test", "c", "d.com")

	if web != unknown {
		t.Error("unknown template type should default to 'web'")
	}
}

func TestGenerateScoreTemplate_DifferentNames(t *testing.T) {
	templates := []string{"web", "api", "worker", "cron"}
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			result := generateScoreTemplate(tmpl, "unique-name", "my-cluster", "domain.io")
			if !strings.Contains(result, "name: unique-name") {
				t.Errorf("%s template should contain workload name", tmpl)
			}
			if !strings.Contains(result, `hctl.integratn.tech/cluster: "my-cluster"`) {
				t.Errorf("%s template should contain cluster", tmpl)
			}
		})
	}
}

func TestGenerateScoreTemplate_AllHaveAPIVersion(t *testing.T) {
	templates := []string{"web", "api", "worker", "cron"}
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			result := generateScoreTemplate(tmpl, "app", "cluster", "domain.io")
			if !strings.Contains(result, "apiVersion: score.dev/v1b1") {
				t.Errorf("%s template missing apiVersion", tmpl)
			}
		})
	}
}

func TestGenerateScoreTemplate_AllHaveResourceLimits(t *testing.T) {
	templates := []string{"web", "api", "worker", "cron"}
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			result := generateScoreTemplate(tmpl, "app", "cluster", "domain.io")
			if !strings.Contains(result, "limits:") {
				t.Errorf("%s template missing resource limits", tmpl)
			}
			if !strings.Contains(result, "requests:") {
				t.Errorf("%s template missing resource requests", tmpl)
			}
		})
	}
}
