package runtime

import (
	"context"
	"io"
)

// ContainerRuntime abstracts the container runtime (Docker or Podman).
type ContainerRuntime interface {
	Info(ctx context.Context) (*RuntimeInfo, error)

	CreateContainer(ctx context.Context, config *ContainerConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string, timeout int) error
	RemoveContainer(ctx context.Context, id string, force bool) error
	ListContainers(ctx context.Context, filters map[string]string) ([]Container, error)
	InspectContainer(ctx context.Context, id string) (*ContainerDetail, error)

	ExecInContainer(ctx context.Context, id string, cmd []string, opts ExecOptions) (*ExecResult, error)

	ContainerLogs(ctx context.Context, id string, opts LogOptions) (io.ReadCloser, error)

	CreateNetwork(ctx context.Context, name string, opts NetworkOptions) (string, error)
	RemoveNetwork(ctx context.Context, name string) error
	ConnectNetwork(ctx context.Context, networkID, containerID string) error

	PullImage(ctx context.Context, image string) error
	ImageExists(ctx context.Context, image string) (bool, error)

	CreateVolume(ctx context.Context, name string) (string, error)
	RemoveVolume(ctx context.Context, name string) error
}

type RuntimeInfo struct {
	Name       string
	Version    string
	APIVersion string
}

type ContainerConfig struct {
	Name       string
	Image      string
	Command    []string
	Env        []string
	Labels     map[string]string
	Ports      []PortMapping
	Volumes    []VolumeMount
	Networks   []string
	Privileged bool
	CPULimit   float64
	MemLimit   int64
}

type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

type VolumeMount struct {
	Source   string
	Target  string
	ReadOnly bool
}

type Container struct {
	ID     string
	Name   string
	Image  string
	Status string
	State  string
	Labels map[string]string
}

type ContainerDetail struct {
	Container
	NetworkSettings NetworkSettings
	Mounts          []Mount
}

type NetworkSettings struct {
	IPAddress string
	Networks  map[string]NetworkEndpoint
}

type NetworkEndpoint struct {
	IPAddress string
	Gateway   string
}

type Mount struct {
	Source      string
	Destination string
	ReadOnly    bool
}

type NetworkOptions struct {
	Driver string
	Subnet string
	Labels map[string]string
}

type ExecOptions struct {
	Interactive bool
	TTY         bool
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

type ExecResult struct {
	ExitCode int
	Output   string
}

type LogOptions struct {
	Follow     bool
	Tail       string
	Since      string
	Timestamps bool
}
