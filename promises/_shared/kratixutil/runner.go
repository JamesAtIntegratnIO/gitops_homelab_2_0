package kratixutil

import (
	"fmt"
	"log"

	kratix "github.com/syntasso/kratix-go"
)

// RunPromiseWithConfigE is the error-returning version of RunPromiseWithConfig.
// It is suitable for unit testing because it returns errors instead of calling
// log.Fatalf (which invokes os.Exit).
func RunPromiseWithConfigE[T any](
	name string,
	buildConfig func(*kratix.KratixSDK, kratix.Resource) (T, error),
	configure func(*kratix.KratixSDK, T) error,
	delete func(*kratix.KratixSDK, T) error,
) error {
	sdk := kratix.New()

	log.Printf("=== %s Pipeline ===", name)
	log.Printf("Action: %s", sdk.WorkflowAction())

	resource, err := sdk.ReadResourceInput()
	if err != nil {
		return fmt.Errorf("read resource input: %w", err)
	}

	log.Printf("Processing resource: %s/%s",
		resource.GetNamespace(), resource.GetName())

	config, err := buildConfig(sdk, resource)
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}

	switch sdk.WorkflowAction() {
	case "configure":
		if err := configure(sdk, config); err != nil {
			return fmt.Errorf("configure: %w", err)
		}
	case "delete":
		if err := delete(sdk, config); err != nil {
			return fmt.Errorf("delete: %w", err)
		}
	default:
		return fmt.Errorf("unknown action: %s", sdk.WorkflowAction())
	}

	log.Println("=== Pipeline completed successfully ===")
	return nil
}

// RunPromiseWithConfig runs a promise pipeline with a typed config build step.
// buildConfig extracts configuration from the Kratix resource.
// configure and delete receive the SDK and the built config.
//
// log.Fatalf is intentional here: this is the top-level entry point called
// from main(), so a fatal exit on error is the correct Go idiom.
func RunPromiseWithConfig[T any](
	name string,
	buildConfig func(*kratix.KratixSDK, kratix.Resource) (T, error),
	configure func(*kratix.KratixSDK, T) error,
	delete func(*kratix.KratixSDK, T) error,
) {
	if err := RunPromiseWithConfigE(name, buildConfig, configure, delete); err != nil {
		log.Fatalf("%v", err)
	}
}
