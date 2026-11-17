package podman

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/torresmo-dev/torresmo/internal/runtime"
)

// Client implements runtime.ContainerRuntime using Podman's Docker-compatible API.
type Client struct {
	cli *client.Client
}

// Compile-time check that Client implements runtime.ContainerRuntime.
var _ runtime.ContainerRuntime = (*Client)(nil)

// New creates a Podman client by connecting to the Podman socket.
func New(ctx context.Context) (*Client, error) {
	socketPath, err := findPodmanSocket()
	if err != nil {
		return nil, fmt.Errorf("podman: %w", err)
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	cli, err := client.NewClientWithOpts(
		client.WithHost("unix://"+socketPath),
		client.WithHTTPClient(httpClient),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("podman: failed to create client: %w", err)
	}

	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("podman: daemon not responding at %s: %w", socketPath, err)
	}

	return &Client{cli: cli}, nil
}

// findPodmanSocket locates the Podman API socket.
func findPodmanSocket() (string, error) {
	// 1. Check DOCKER_HOST env (podman can set this).
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		if path, ok := strings.CutPrefix(host, "unix://"); ok {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// 2. Check CONTAINER_HOST env (podman-specific).
	if host := os.Getenv("CONTAINER_HOST"); host != "" {
		if path, ok := strings.CutPrefix(host, "unix://"); ok {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// 3. Check XDG_RUNTIME_DIR (Linux rootless).
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		path := filepath.Join(xdg, "podman", "podman.sock")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// 4. /run/user/{UID}/podman/podman.sock (Linux rootless).
	uid := os.Getuid()
	path := fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// 5. /run/podman/podman.sock (Linux rootful).
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "/run/podman/podman.sock", nil
	}

	// 6. macOS: try podman machine inspect to find socket.
	if out, err := exec.Command("podman", "machine", "inspect", "--format", "{{.ConnectionInfo.PodmanSocket.Path}}").Output(); err == nil {
		sockPath := strings.TrimSpace(string(out))
		if sockPath != "" {
			if _, err := os.Stat(sockPath); err == nil {
				return sockPath, nil
			}
		}
	}

	// 7. macOS common paths.
	home, _ := os.UserHomeDir()
	if home != "" {
		macPaths := []string{
			filepath.Join(home, ".local", "share", "containers", "podman", "machine", "podman.sock"),
			filepath.Join(home, ".local", "share", "containers", "podman", "machine", "qemu", "podman.sock"),
		}
		for _, p := range macPaths {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("podman socket not found; ensure Podman is running:\n  podman machine start  (macOS)\n  systemctl --user start podman.socket  (Linux)")
}

// Info returns runtime details.
func (c *Client) Info(ctx context.Context) (*runtime.RuntimeInfo, error) {
	info, err := c.cli.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("podman: failed to get info: %w", err)
	}

	return &runtime.RuntimeInfo{
		Name:       "podman",
		Version:    info.ServerVersion,
		APIVersion: c.cli.ClientVersion(),
	}, nil
}

// CreateContainer creates a container from the given config and returns its ID.
func (c *Client) CreateContainer(ctx context.Context, config *runtime.ContainerConfig) (string, error) {
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}

	for _, p := range config.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		containerPort, err := nat.NewPort(proto, strconv.Itoa(p.ContainerPort))
		if err != nil {
			return "", fmt.Errorf("podman: invalid port %d/%s: %w", p.ContainerPort, proto, err)
		}
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []nat.PortBinding{
			{HostPort: strconv.Itoa(p.HostPort)},
		}
	}

	var binds []string
	for _, v := range config.Volumes {
		bind := v.Source + ":" + v.Target
		if v.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	containerCfg := &container.Config{
		Image:        config.Image,
		Cmd:          config.Command,
		Env:          config.Env,
		Labels:       config.Labels,
		ExposedPorts: exposedPorts,
	}

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        binds,
		Privileged:   config.Privileged,
	}

	if config.CPULimit > 0 {
		hostCfg.NanoCPUs = int64(config.CPULimit * 1e9)
	}
	if config.MemLimit > 0 {
		hostCfg.Memory = config.MemLimit
	}

	networkCfg := &network.NetworkingConfig{}
	if len(config.Networks) > 0 {
		networkCfg.EndpointsConfig = make(map[string]*network.EndpointSettings, len(config.Networks))
		for _, n := range config.Networks {
			networkCfg.EndpointsConfig[n] = &network.EndpointSettings{}
		}
	}

	resp, err := c.cli.ContainerCreate(ctx, containerCfg, hostCfg, networkCfg, nil, config.Name)
	if err != nil {
		return "", fmt.Errorf("podman: failed to create container %q: %w", config.Name, err)
	}

	return resp.ID, nil
}

// StartContainer starts a container by ID.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return fmt.Errorf("podman: failed to start container %q: %w", id, err)
	}
	return nil
}

// StopContainer stops a container with the given timeout in seconds.
func (c *Client) StopContainer(ctx context.Context, id string, timeout int) error {
	stopOpts := container.StopOptions{
		Timeout: &timeout,
	}
	if err := c.cli.ContainerStop(ctx, id, stopOpts); err != nil {
		return fmt.Errorf("podman: failed to stop container %q: %w", id, err)
	}
	return nil
}

// RemoveContainer removes a container, optionally with force.
func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	if err := c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force}); err != nil {
		return fmt.Errorf("podman: failed to remove container %q: %w", id, err)
	}
	return nil
}

// ListContainers lists containers matching the given label filters.
func (c *Client) ListContainers(ctx context.Context, labelFilters map[string]string) ([]runtime.Container, error) {
	args := filters.NewArgs()
	for k, v := range labelFilters {
		args.Add("label", k+"="+v)
	}

	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("podman: failed to list containers: %w", err)
	}

	result := make([]runtime.Container, 0, len(containers))
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = ctr.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}
		result = append(result, runtime.Container{
			ID:     ctr.ID,
			Name:   name,
			Image:  ctr.Image,
			Status: ctr.Status,
			State:  ctr.State,
			Labels: ctr.Labels,
		})
	}

	return result, nil
}

// InspectContainer inspects a container and returns its details.
func (c *Client) InspectContainer(ctx context.Context, id string) (*runtime.ContainerDetail, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("podman: failed to inspect container %q: %w", id, err)
	}

	name := info.Name
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}

	detail := &runtime.ContainerDetail{
		Container: runtime.Container{
			ID:     info.ID,
			Name:   name,
			Image:  info.Config.Image,
			Status: info.State.Status,
			State:  info.State.Status,
			Labels: info.Config.Labels,
		},
	}

	if info.NetworkSettings != nil {
		var defaultIP string
		for _, ep := range info.NetworkSettings.Networks {
			defaultIP = ep.IPAddress
			break
		}
		detail.NetworkSettings.IPAddress = defaultIP
		if len(info.NetworkSettings.Networks) > 0 {
			detail.NetworkSettings.Networks = make(map[string]runtime.NetworkEndpoint, len(info.NetworkSettings.Networks))
			for netName, ep := range info.NetworkSettings.Networks {
				detail.NetworkSettings.Networks[netName] = runtime.NetworkEndpoint{
					IPAddress: ep.IPAddress,
					Gateway:   ep.Gateway,
				}
			}
		}
	}

	for _, m := range info.Mounts {
		detail.Mounts = append(detail.Mounts, runtime.Mount{
			Source:      m.Source,
			Destination: m.Destination,
			ReadOnly:    !m.RW,
		})
	}

	return detail, nil
}

// ExecInContainer creates and runs an exec instance inside a container.
func (c *Client) ExecInContainer(ctx context.Context, id string, cmd []string, opts runtime.ExecOptions) (*runtime.ExecResult, error) {
	execCfg := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          opts.TTY,
	}

	if opts.Interactive {
		execCfg.AttachStdin = true
	}

	execResp, err := c.cli.ContainerExecCreate(ctx, id, execCfg)
	if err != nil {
		return nil, fmt.Errorf("podman: failed to create exec in container %q: %w", id, err)
	}

	attachResp, err := c.cli.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{Tty: opts.TTY})
	if err != nil {
		return nil, fmt.Errorf("podman: failed to attach exec in container %q: %w", id, err)
	}

	if opts.Interactive && opts.TTY {
		done := make(chan struct{})

		if opts.Stdin != nil {
			go func() {
				_, _ = io.Copy(attachResp.Conn, opts.Stdin)
				_ = attachResp.CloseWrite()
			}()
		}

		go func() {
			defer close(done)
			dst := opts.Stdout
			if dst == nil {
				dst = io.Discard
			}
			_, _ = io.Copy(dst, attachResp.Reader)
		}()

		<-done
		attachResp.Close()

		inspect, err := c.cli.ContainerExecInspect(ctx, execResp.ID)
		if err != nil {
			return nil, fmt.Errorf("podman: failed to inspect exec result: %w", err)
		}

		return &runtime.ExecResult{ExitCode: inspect.ExitCode}, nil
	}

	defer attachResp.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, attachResp.Reader); err != nil {
		return nil, fmt.Errorf("podman: failed to read exec output: %w", err)
	}

	inspect, err := c.cli.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return nil, fmt.Errorf("podman: failed to inspect exec result: %w", err)
	}

	return &runtime.ExecResult{
		ExitCode: inspect.ExitCode,
		Output:   buf.String(),
	}, nil
}

// ContainerLogs returns a log stream for a container.
func (c *Client) ContainerLogs(ctx context.Context, id string, opts runtime.LogOptions) (io.ReadCloser, error) {
	logOpts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Timestamps: opts.Timestamps,
	}
	if opts.Tail != "" {
		logOpts.Tail = opts.Tail
	}
	if opts.Since != "" {
		logOpts.Since = opts.Since
	}

	reader, err := c.cli.ContainerLogs(ctx, id, logOpts)
	if err != nil {
		return nil, fmt.Errorf("podman: failed to get logs for container %q: %w", id, err)
	}

	return reader, nil
}

// CreateNetwork creates a network and returns its ID.
func (c *Client) CreateNetwork(ctx context.Context, name string, opts runtime.NetworkOptions) (string, error) {
	createOpts := network.CreateOptions{
		Driver: opts.Driver,
		Labels: opts.Labels,
	}

	if opts.Subnet != "" {
		createOpts.IPAM = &network.IPAM{
			Config: []network.IPAMConfig{
				{Subnet: opts.Subnet},
			},
		}
	}

	resp, err := c.cli.NetworkCreate(ctx, name, createOpts)
	if err != nil {
		return "", fmt.Errorf("podman: failed to create network %q: %w", name, err)
	}

	return resp.ID, nil
}

// RemoveNetwork removes a network by name.
func (c *Client) RemoveNetwork(ctx context.Context, name string) error {
	if err := c.cli.NetworkRemove(ctx, name); err != nil {
		return fmt.Errorf("podman: failed to remove network %q: %w", name, err)
	}
	return nil
}

// ConnectNetwork connects a container to a network.
func (c *Client) ConnectNetwork(ctx context.Context, networkID, containerID string) error {
	if err := c.cli.NetworkConnect(ctx, networkID, containerID, nil); err != nil {
		return fmt.Errorf("podman: failed to connect container %q to network %q: %w", containerID, networkID, err)
	}
	return nil
}

// PullImage pulls a container image.
func (c *Client) PullImage(ctx context.Context, img string) error {
	reader, err := c.cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("podman: failed to pull image %q: %w", img, err)
	}
	defer func() { _ = reader.Close() }()

	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("podman: error reading pull response for %q: %w", img, err)
	}

	return nil
}

// ImageExists checks whether an image exists locally.
func (c *Client) ImageExists(ctx context.Context, img string) (bool, error) {
	_, err := c.cli.ImageInspect(ctx, img)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("podman: failed to inspect image %q: %w", img, err)
	}
	return true, nil
}

// CreateVolume creates a named volume and returns its mount path.
func (c *Client) CreateVolume(ctx context.Context, name string) (string, error) {
	v, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return "", fmt.Errorf("podman: failed to create volume %q: %w", name, err)
	}
	return v.Mountpoint, nil
}

// RemoveVolume removes a volume by name.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	if err := c.cli.VolumeRemove(ctx, name, true); err != nil {
		return fmt.Errorf("podman: failed to remove volume %q: %w", name, err)
	}
	return nil
}
