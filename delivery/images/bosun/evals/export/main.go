// Command export prints the eval cases as JSON.
//
// The cases are the platform's real incidents, already written down once as a
// Go fixture. Rather than describe them a second time in shell, the live
// scenario demo reads them from here -- so the thing the eval measures and the
// thing the demo shows can never drift apart.
package main

import (
	"encoding/json"
	"os"

	"github.com/JamesAtIntegratnIO/delivery-kit/bosun/evals"
)

func main() {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(evals.Cases); err != nil {
		os.Exit(1)
	}
}
