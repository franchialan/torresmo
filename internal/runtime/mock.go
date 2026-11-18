package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
)

// MockRuntime is a test implementation of ContainerRuntime.
type MockRuntime struct {
	mu         sync.Mutex
	containers map[string]*MockContainer
	networks   map[string]bool
	images     map[string]bool
	idCounter  atomic.Int64
}

// MockContainer holds the state of a mock container.
type MockContainer struct {
	ID     string
	Config *ContainerConfig
	State  string // "created", "running", "stopped"
	IP     string
}

// NewMockRuntime creates a MockRuntime pre-loaded with common images.
func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		containers: make(map[string]*MockContainer),
		networks:   make(map[string]bool),
		images:     map[string]bool{"alpine:3.19": true, "torresmo/node:latest": true, "redis:7.2-alpine": true, "postgres:16-alpine": true},
	}
}

func (m *MockRuntime) Info(_ context.Context) (*RuntimeInfo, error) {
	return &RuntimeInfo{Name: "mock", Version: "1.0.0", APIVersion: "1.0"}, nil
}

func (m *MockRuntime) CreateContainer(_ context.Context, config *ContainerConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := fmt.Sprintf("mock-%d", m.idCounter.Add(1))
	m.containers[id] = &MockContainer{
		ID:     id,
		Config: config,
		State:  "created",
		IP:     fmt.Sprintf("10.244.0.%d", m.idCounter.Load()+1),
	}
	return id, nil
}

func (m *MockRuntime) StartContainer(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container %s not found", id)
	}
	c.State = "running"
	return nil
}

func (m *MockRuntime) StopContainer(_ context.Context, id string, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container %s not found", id)
	}
	c.State = "stopped"
	return nil
}

func (m *MockRuntime) RemoveContainer(_ context.Context, id string, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.containers, id)
	return nil
}

func (m *MockRuntime) ListContainers(_ context.Context, filters map[string]string) ([]Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Container
	for _, c := range m.containers {
		match := true
		for k, v := range filters {
			if c.Config.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			result = append(result, Container{
				ID:     c.ID,
				Name:   c.Config.Name,
				Image:  c.Config.Image,
				Status: c.State,
				State:  c.State,
				Labels: c.Config.Labels,
			})
		}
	}
	return result, nil
}

func (m *MockRuntime) InspectContainer(_ context.Context, id string) (*ContainerDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container %s not found", id)
	}
	networkName := ""
	if len(c.Config.Networks) > 0 {
		networkName = c.Config.Networks[0]
	}
	return &ContainerDetail{
		Container: Container{
			ID:     c.ID,
			Name:   c.Config.Name,
			Image:  c.Config.Image,
			Status: c.State,
			State:  c.State,
			Labels: c.Config.Labels,
		},
		NetworkSettings: NetworkSettings{
			IPAddress: c.IP,
			Networks: map[string]NetworkEndpoint{
				networkName: {IPAddress: c.IP, Gateway: "10.244.0.1"},
			},
		},
	}, nil
}

func (m *MockRuntime) ExecInContainer(_ context.Context, id string, _ []string, _ ExecOptions) (*ExecResult, error) {
	m.mu.Lock()
	_, ok := m.containers[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("container %s not found", id)
	}
	return &ExecResult{ExitCode: 0, Output: ""}, nil
}

func (m *MockRuntime) ContainerLogs(_ context.Context, _ string, _ LogOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("mock log output\n")), nil
}

func (m *MockRuntime) CreateNetwork(_ context.Context, name string, _ NetworkOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.networks[name] = true
	return name, nil
}

func (m *MockRuntime) RemoveNetwork(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.networks, name)
	return nil
}

func (m *MockRuntime) ConnectNetwork(_ context.Context, _, _ string) error {
	return nil
}

func (m *MockRuntime) PullImage(_ context.Context, image string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.images[image] = true
	return nil
}

func (m *MockRuntime) ImageExists(_ context.Context, image string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.images[image], nil
}

func (m *MockRuntime) CreateVolume(_ context.Context, name string) (string, error) {
	return "/volumes/" + name, nil
}

func (m *MockRuntime) RemoveVolume(_ context.Context, _ string) error {
	return nil
}

// ContainerCount returns the number of tracked containers (test helper).
func (m *MockRuntime) ContainerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.containers)
}

// NetworkExists reports whether a network has been created (test helper).
func (m *MockRuntime) NetworkExists(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.networks[name]
}
