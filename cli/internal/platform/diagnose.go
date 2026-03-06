package platform

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DiagnosticResult holds the results of a platform diagnostic check.
type DiagnosticResult struct {
	Steps []DiagnosticStep
}

// DiagnosticStep represents one step in the diagnostic chain.
type DiagnosticStep struct {
	Name    string
	Status  StepStatus
	Message string
	Details string
}

// StepStatus represents the health status of a diagnostic step.
type StepStatus int

const (
	StatusOK StepStatus = iota
	StatusWarning
	StatusError
	StatusUnknown
)

func (s StepStatus) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusWarning:
		return "warning"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// DiagnosticChecker performs a single diagnostic check as part of a chain.
type DiagnosticChecker interface {
	// Check runs the diagnostic and returns steps plus whether further checks
	// should be halted (e.g. when the resource is not found).
	Check(ctx context.Context, client KubeClient, state *DiagnosticState) (steps []DiagnosticStep, halt bool)
}

// DiagnosticState holds shared state accumulated across diagnostic checkers.
type DiagnosticState struct {
	Namespace string
	Name      string
	TargetNS  string
	VCluster  *unstructured.Unstructured
}

// RunDiagnostics executes a sequence of diagnostic checkers, short-circuiting
// if any checker signals halt.
func RunDiagnostics(ctx context.Context, client KubeClient, checkers []DiagnosticChecker, state *DiagnosticState) *DiagnosticResult {
	result := &DiagnosticResult{}
	for _, c := range checkers {
		steps, halt := c.Check(ctx, client, state)
		result.Steps = append(result.Steps, steps...)
		if halt {
			break
		}
	}
	return result
}

// DefaultCheckers returns the standard set of vCluster diagnostic checkers.
func DefaultCheckers() []DiagnosticChecker {
	return []DiagnosticChecker{
		&ResourceRequestChecker{},
		&PipelineJobChecker{},
		&WorkChecker{},
		&WorkPlacementChecker{},
		&ArgoCDAppChecker{},
		&PodChecker{},
		&PodResourceChecker{},
		&SubAppHealthChecker{},
	}
}

// DiagnoseVCluster runs the full diagnostic chain for a vCluster resource.
func DiagnoseVCluster(ctx context.Context, client KubeClient, namespace, name string) (*DiagnosticResult, error) {
	state := &DiagnosticState{
		Namespace: namespace,
		Name:      name,
		TargetNS:  name,
	}
	return RunDiagnostics(ctx, client, DefaultCheckers(), state), nil
}

