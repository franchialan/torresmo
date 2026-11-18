package cluster

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContextName returns the torresmo context name for a cluster.
func ContextName(clusterName string) string {
	return "torresmo-" + clusterName
}

// contextPath returns the path to the context file.
func contextPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".torresmo", "context"), nil
}

// SetContext sets the active cluster context.
func SetContext(clusterName string) error {
	path, err := contextPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(clusterName), 0o644)
}

// GetContext returns the active cluster name, or empty if none set.
func GetContext() string {
	path, err := contextPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// ClearContext removes the active context.
func ClearContext() error {
	path, err := contextPath()
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// PrintContext prints the current context info.
func PrintContext(clusterName string) {
	fmt.Printf("  Context: %s\n", ContextName(clusterName))
}
