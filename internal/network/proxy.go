package network

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
)

// PortForward represents an active port-forward session.
type PortForward struct {
	LocalPort  int
	RemoteAddr string // ip:port inside cluster
	listener   net.Listener
	cancel     context.CancelFunc
	done       chan struct{}
}

// ProxyManager handles port-forwarding from host to cluster.
type ProxyManager struct {
	forwards map[string]*PortForward // key: "localPort"
	mu       sync.Mutex
}

func NewProxyManager() *ProxyManager {
	return &ProxyManager{
		forwards: make(map[string]*PortForward),
	}
}

// Forward creates a port-forward from localhost:localPort to remoteAddr (ip:port).
func (p *ProxyManager) Forward(ctx context.Context, localPort int, remoteAddr string) (*PortForward, error) {
	key := fmt.Sprintf("%d", localPort)

	p.mu.Lock()
	if _, exists := p.forwards[key]; exists {
		p.mu.Unlock()
		return nil, fmt.Errorf("port %d is already forwarded", localPort)
	}
	p.mu.Unlock()

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", localPort, err)
	}

	ctx, cancel := context.WithCancel(ctx)
	pf := &PortForward{
		LocalPort:  localPort,
		RemoteAddr: remoteAddr,
		listener:   listener,
		cancel:     cancel,
		done:       make(chan struct{}),
	}

	p.mu.Lock()
	p.forwards[key] = pf
	p.mu.Unlock()

	go p.serve(ctx, pf)

	return pf, nil
}

func (p *ProxyManager) serve(ctx context.Context, pf *PortForward) {
	defer close(pf.done)
	defer func() { _ = pf.listener.Close() }()

	for {
		conn, err := pf.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		go p.handleConn(ctx, conn, pf.RemoteAddr)
	}
}

func (p *ProxyManager) handleConn(_ context.Context, local net.Conn, remoteAddr string) {
	defer func() { _ = local.Close() }()

	remote, err := net.Dial("tcp", remoteAddr)
	if err != nil {
		return
	}
	defer func() { _ = remote.Close() }()

	var wg sync.WaitGroup
	wg.Add(2)

	// local -> remote
	go func() {
		defer wg.Done()
		_, _ = io.Copy(remote, local)
	}()

	// remote -> local
	go func() {
		defer wg.Done()
		_, _ = io.Copy(local, remote)
	}()

	wg.Wait()
}

// Stop stops a port-forward.
func (p *ProxyManager) Stop(localPort int) error {
	key := fmt.Sprintf("%d", localPort)

	p.mu.Lock()
	pf, exists := p.forwards[key]
	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("no forward on port %d", localPort)
	}
	delete(p.forwards, key)
	p.mu.Unlock()

	pf.cancel()
	<-pf.done
	return nil
}

// StopAll stops all port-forwards.
func (p *ProxyManager) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, pf := range p.forwards {
		pf.cancel()
		<-pf.done
		delete(p.forwards, key)
	}
}

// List returns all active port-forwards.
func (p *ProxyManager) List() []PortForward {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make([]PortForward, 0, len(p.forwards))
	for _, pf := range p.forwards {
		result = append(result, PortForward{
			LocalPort:  pf.LocalPort,
			RemoteAddr: pf.RemoteAddr,
		})
	}
	return result
}
