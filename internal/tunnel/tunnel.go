package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
)

// Tunnel represents an active tunnel.
type Tunnel struct {
	LocalPort  int
	RemoteAddr string // cluster pod ip:port
	PublicURL  string // public URL (when using external provider)
	Provider   string // "local", "cloudflared", "bore"
	listener   net.Listener
	cancel     context.CancelFunc
	done       chan struct{}
}

// Manager handles tunnel lifecycle.
type Manager struct {
	tunnels map[int]*Tunnel
	mu      sync.Mutex
}

func NewManager() *Manager {
	return &Manager{
		tunnels: make(map[int]*Tunnel),
	}
}

// CreateLocal creates a local port-forward tunnel (localhost -> pod).
func (m *Manager) CreateLocal(ctx context.Context, localPort int, remoteAddr string) (*Tunnel, error) {
	m.mu.Lock()
	if _, exists := m.tunnels[localPort]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("port %d is already tunneled", localPort)
	}
	m.mu.Unlock()

	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", localPort, err)
	}

	ctx, cancel := context.WithCancel(ctx)
	t := &Tunnel{
		LocalPort:  localPort,
		RemoteAddr: remoteAddr,
		Provider:   "local",
		PublicURL:  fmt.Sprintf("http://localhost:%d", localPort),
		listener:   listener,
		cancel:     cancel,
		done:       make(chan struct{}),
	}

	m.mu.Lock()
	m.tunnels[localPort] = t
	m.mu.Unlock()

	go m.serve(ctx, t)
	return t, nil
}

func (m *Manager) serve(ctx context.Context, t *Tunnel) {
	defer close(t.done)
	defer t.listener.Close()

	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		go m.handleConn(conn, t.RemoteAddr)
	}
}

func (m *Manager) handleConn(local net.Conn, remoteAddr string) {
	defer local.Close()

	remote, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		return
	}
	defer remote.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(remote, local) }()
	go func() { defer wg.Done(); io.Copy(local, remote) }()
	wg.Wait()
}

// CreateCloudflared creates a tunnel using cloudflared (if installed).
func (m *Manager) CreateCloudflared(ctx context.Context, localPort int, remoteAddr string) (*Tunnel, error) {
	if _, err := exec.LookPath("cloudflared"); err != nil {
		return nil, fmt.Errorf("cloudflared not found. Install from https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/")
	}

	t, err := m.CreateLocal(ctx, localPort, remoteAddr)
	if err != nil {
		return nil, err
	}
	t.Provider = "cloudflared"

	cmd := exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", localPort))

	output, err := cmd.StdoutPipe()
	if err == nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, readErr := output.Read(buf)
				if readErr != nil {
					return
				}
				line := string(buf[:n])
				if url := findURL(line, "trycloudflare.com"); url != "" {
					t.PublicURL = url
				}
			}
		}()
	}

	go cmd.Run()

	return t, nil
}

// findURL extracts a URL containing the given domain from text.
func findURL(text, domain string) string {
	for {
		idx := strings.Index(text, "https://")
		if idx < 0 {
			return ""
		}
		text = text[idx:]
		end := strings.IndexAny(text, " \n\"")
		if end < 0 {
			end = len(text)
		}
		url := text[:end]
		if strings.Contains(url, domain) {
			return url
		}
		text = text[end:]
	}
}

// Stop closes a tunnel.
func (m *Manager) Stop(localPort int) error {
	m.mu.Lock()
	t, ok := m.tunnels[localPort]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("no tunnel on port %d", localPort)
	}
	delete(m.tunnels, localPort)
	m.mu.Unlock()

	t.cancel()
	<-t.done
	return nil
}

// StopAll closes all tunnels.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for port, t := range m.tunnels {
		t.cancel()
		<-t.done
		delete(m.tunnels, port)
	}
}

// List returns all active tunnels.
func (m *Manager) List() []*Tunnel {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Tunnel, 0, len(m.tunnels))
	for _, t := range m.tunnels {
		result = append(result, t)
	}
	return result
}

// DetectProvider checks which tunnel providers are available.
func DetectProvider() string {
	if _, err := exec.LookPath("cloudflared"); err == nil {
		return "cloudflared"
	}
	if _, err := exec.LookPath("bore"); err == nil {
		return "bore"
	}
	return "local"
}
