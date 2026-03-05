package kratixutil

import (
	"log"

	kratix "github.com/syntasso/kratix-go"
)

// RunPromise is a shared entry point for simple Kratix promise pipelines.
// It handles SDK initialization, resource reading, and dispatches to the
// supplied configure/delete handlers based on the workflow action.
//
// This is suitable for promises that do not require a buildConfig step
// and operate directly on the kratix.Resource. Promises that need custom
// config construction should use their own main() function.
func RunPromise(
	banner string,
	configure func(sdk *kratix.KratixSDK, resource kratix.Resource) error,
	delete func(sdk *kratix.KratixSDK, resource kratix.Resource) error,
) {
	sdk := kratix.New()

	log.Printf("=== %s ===", banner)
	log.Printf("Action: %s", sdk.WorkflowAction())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		log.Fatalf("ERROR: Failed to read resource input: %v", err)
	}

	log.Printf("Processing resource: %s/%s",
		resource.GetNamespace(), resource.GetName())

	switch sdk.WorkflowAction() {
	case "configure":
		if err := configure(sdk, resource); err != nil {
			log.Fatalf("ERROR: Configure failed: %v", err)
		}
	case "delete":
		if err := delete(sdk, resource); err != nil {
			log.Fatalf("ERROR: Delete failed: %v", err)
		}
	default:
		log.Fatalf("ERROR: Unknown workflow action: %s", sdk.WorkflowAction())
	}

	log.Println("=== Pipeline completed successfully ===")
}
