package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openBrowser opens the given URL in the default system browser.
func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported OS for browser open: %s", runtime.GOOS)
	}
	return exec.Command(cmd, args...).Start()
}
