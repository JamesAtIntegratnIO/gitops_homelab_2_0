package main

import (
	"testing"
)

// TestMainCompiles verifies that the main package compiles and the main
// function symbol exists. This is a build-verification test — it does not
// execute main() because that would call os.Exit and interfere with testing.
func TestMainCompiles(t *testing.T) {
	// If this test runs, the package compiled successfully.
	// Verify the main function is callable (take its address).
	fn := main
	if fn == nil {
		t.Fatal("main function should not be nil")
	}
}
