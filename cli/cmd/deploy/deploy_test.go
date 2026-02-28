package deploy

import (
	"strings"
	"testing"
)

func TestGenerateScoreTemplateWeb(t *testing.T) {
	out := generateScoreTemplate("web", "myapp", "dev-cluster", "example.com")
	if !strings.Contains(out, "name: myapp") {
		t.Error("web template should contain workload name")
	}
	if !strings.Contains(out, `hctl.integratn.tech/cluster: "dev-cluster"`) {
		t.Error("web template should contain cluster annotation")
	}
	if !strings.Contains(out, "type: route") {
		t.Error("web template should include route resource")
	}
	if !strings.Contains(out, "myapp.example.com") {
		t.Error("web template should include domain-based host")
	}
	if !strings.Contains(out, "/healthz") {
		t.Error("web template should include health check")
	}
}

func TestGenerateScoreTemplateAPI(t *testing.T) {
	out := generateScoreTemplate("api", "myapi", "staging", "test.io")
	if !strings.Contains(out, "name: myapi") {
		t.Error("api template should contain workload name")
	}
	if !strings.Contains(out, "type: postgres") {
		t.Error("api template should include postgres resource")
	}
	if !strings.Contains(out, "type: route") {
		t.Error("api template should include route resource")
	}
	if !strings.Contains(out, "${resources.db.host}") {
		t.Error("api template should reference db outputs")
	}
}

func TestGenerateScoreTemplateWorker(t *testing.T) {
	out := generateScoreTemplate("worker", "processor", "prod", "example.com")
	if !strings.Contains(out, "name: processor") {
		t.Error("worker template should contain workload name")
	}
	// Workers should NOT have a route
	if strings.Contains(out, "type: route") {
		t.Error("worker template should NOT include route resource")
	}
	if !strings.Contains(out, "worker:") {
		t.Error("worker template should name the container 'worker'")
	}
}

func TestGenerateScoreTemplateCron(t *testing.T) {
	out := generateScoreTemplate("cron", "cleanup", "dev", "example.com")
	if !strings.Contains(out, "name: cleanup") {
		t.Error("cron template should contain workload name")
	}
	if !strings.Contains(out, "command:") {
		t.Error("cron template should include command")
	}
	// Cron should have no service/route
	if strings.Contains(out, "type: route") {
		t.Error("cron template should NOT include route")
	}
	if !strings.Contains(out, "resources: {}") {
		t.Error("cron template should have empty resources")
	}
}

func TestGenerateScoreTemplateDefaultIsWeb(t *testing.T) {
	webOut := generateScoreTemplate("web", "app", "cluster", "example.com")
	defaultOut := generateScoreTemplate("unknown-template", "app", "cluster", "example.com")
	if webOut != defaultOut {
		t.Error("unknown template should fall back to web")
	}
}

func TestAllTemplatesHaveRequiredFields(t *testing.T) {
	templates := []string{"web", "api", "worker", "cron"}
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			out := generateScoreTemplate(tmpl, "test", "cluster", "example.com")
			if !strings.Contains(out, "apiVersion: score.dev/v1b1") {
				t.Error("template should contain apiVersion")
			}
			if !strings.Contains(out, "name: test") {
				t.Error("template should contain workload name")
			}
			if !strings.Contains(out, "image:") {
				t.Error("template should contain image field")
			}
		})
	}
}
