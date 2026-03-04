package tui

import "testing"

func TestSetVerbose(t *testing.T) {
	SetVerbose(true)
	if !verbose {
		t.Error("verbose should be true after SetVerbose(true)")
	}
	SetVerbose(false)
	if verbose {
		t.Error("verbose should be false after SetVerbose(false)")
	}
}

func TestSetQuiet(t *testing.T) {
	SetQuiet(true)
	if !quiet {
		t.Error("quiet should be true after SetQuiet(true)")
	}
	SetQuiet(false)
	if quiet {
		t.Error("quiet should be false after SetQuiet(false)")
	}
}

func TestVerboseQuietInteraction(t *testing.T) {
	// Both can be set independently
	SetVerbose(true)
	SetQuiet(true)
	if !verbose {
		t.Error("verbose should be true")
	}
	if !quiet {
		t.Error("quiet should be true")
	}
	// Reset
	SetVerbose(false)
	SetQuiet(false)
}
