package runtime

import (
	"fmt"
	"os/exec"
)

// Detect finds the available container runtime.
func Detect() (string, error) {
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker", nil
	}

	if _, err := exec.LookPath("podman"); err == nil {
		return "podman", nil
	}

	return "", fmt.Errorf("no container runtime found: install Docker or Podman")
}
