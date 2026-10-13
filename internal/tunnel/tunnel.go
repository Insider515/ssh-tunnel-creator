// Package tunnel implements a TCP tunnel through an intermediate (jump) SSH server.
// The jump server connects to target_host:target_port on behalf of the operator's local port.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"sshtunnel/internal/i18n"
)

// Status is a tunnel state displayed in the interface.
type Status string

const (
	// StatusIdle means the tunnel is not running.
	StatusIdle Status = "Idle"
	// StatusChecking means the connection is being checked or established.
	StatusChecking Status = "Checking"
	// StatusActive means the tunnel is running and listening on its local port.
	StatusActive Status = "Active"
	// StatusReconnecting means the jump connection was lost and reconnection is in progress.
	StatusReconnecting Status = "Reconnecting"
	// StatusError means the most recent operation failed.
	StatusError Status = "Error"
)

// Config contains all tunnel parameters.
type Config struct {
	LocalHost string // The local listener address; an empty value means 127.0.0.1.
	LocalPort int

	JumpHost string
	JumpPort int
	JumpUser string
	Auth     AuthConfig

	TargetHost string
	TargetPort int

	VerifyHostKey  bool
	KnownHostsFile string
	HostKeyPrompt  HostKeyPrompt

	ConnectTimeout time.Duration
	KeepAlive      time.Duration
	AutoReconnect  bool
}

// Stats contains connection counters for the interface.
type Stats struct {
	Active int    // Connections currently open.
	Total  uint64 // Connections served since startup.
}

// Callbacks report tunnel state to the interface.
// Callbacks run from worker goroutines; implementations must dispatch UI updates themselves.
type Callbacks struct {
	OnStatus func(Status, string)
	OnLog    func(string)
	OnStats  func(Stats)
}

// Tunnel manages one tunnel's lifecycle: validation, startup, data transfer, and shutdown.
type Tunnel struct {
	cfg Config
	cb  Callbacks

	mu       sync.Mutex
	client   *ssh.Client
	listener net.Listener
	conns    map[io.Closer]struct{}
	cancel   context.CancelFunc
	running  bool

	total atomic.Uint64
}

// New creates a tunnel and supplies defaults for the connection timeout, keepalive, and local address.
func New(cfg Config, cb Callbacks) *Tunnel {
	if cfg.LocalHost == "" {
		cfg.LocalHost = "127.0.0.1"
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 15 * time.Second
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 15 * time.Second
	}
	return &Tunnel{cfg: cfg, cb: cb}
}

// Validate checks parameters before connecting, so invalid fields do not become arbitrary ports
// or unclear network errors.
func (c Config) Validate() error {
	if err := validatePort(i18n.T("Local port"), c.LocalPort); err != nil {
		return err
	}
	if c.JumpHost == "" {
		return errors.New(i18n.T("Jump server address is not set"))
	}
	if err := validatePort(i18n.T("Jump server port"), c.JumpPort); err != nil {
		return err
	}
	if c.JumpUser == "" {
		return errors.New(i18n.T("Jump server username is not set"))
	}
	if c.TargetHost == "" {
		return errors.New(i18n.T("Target server address is not set"))
	}
	if err := validatePort(i18n.T("Target server port"), c.TargetPort); err != nil {
		return err
	}
	switch c.Auth.Method {
	case AuthPassword, AuthKey, AuthAgent:
	default:
		return fmt.Errorf(i18n.T("Unknown authentication method: %q"), c.Auth.Method)
	}
	return nil
}

// validatePort checks that a port is in the valid TCP range.
func validatePort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf(i18n.T("%s must be an integer from 1 to 65535, got %d"), name, port)
	}
	return nil
}

// LocalAddr returns the address listened to by the tunnel on the operator's machine.
func (c Config) LocalAddr() string {
	return net.JoinHostPort(c.LocalHost, strconv.Itoa(c.LocalPort))
}

// JumpAddr returns the intermediate SSH server address.
func (c Config) JumpAddr() string {
	return net.JoinHostPort(c.JumpHost, strconv.Itoa(c.JumpPort))
}

// TargetAddr returns the destination address opened by the jump server.
func (c Config) TargetAddr() string {
	return net.JoinHostPort(c.TargetHost, strconv.Itoa(c.TargetPort))
}

// Running reports whether the tunnel is running.
func (t *Tunnel) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// Check verifies the TCP connection, SSH handshake and host key, authentication, destination
// channel, and local port availability without leaving anything running.
func (t *Tunnel) Check(ctx context.Context) error {
	if err := t.cfg.Validate(); err != nil {
		t.status(StatusError, err.Error())
		return err
	}
	t.status(StatusChecking, i18n.T("Checking connection"))

	client, err := t.connect(ctx)
	if err != nil {
		t.status(StatusError, err.Error())
		return err
	}
	client.Close()

	if !t.Running() {
		ln, err := net.Listen("tcp", t.cfg.LocalAddr())
		if err != nil {
			e := fmt.Errorf(i18n.T("Local address %s is busy: %w"), t.cfg.LocalAddr(), err)
			t.status(StatusError, e.Error())
			return e
		}
		ln.Close()
		t.logf(i18n.T("Local address %s is available"), t.cfg.LocalAddr())
	}

	msg := fmt.Sprintf(i18n.T("Connection check passed: %s is reachable through %s"), t.cfg.TargetAddr(), t.cfg.JumpAddr())
	t.status(StatusIdle, msg)
	return nil
}

// Start verifies the connection before binding the local port and serving clients.
func (t *Tunnel) Start(parent context.Context) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return errors.New(i18n.T("Tunnel is already running"))
	}
	t.mu.Unlock()

	if err := t.cfg.Validate(); err != nil {
		t.status(StatusError, err.Error())
		return err
	}

	ctx, cancel := context.WithCancel(parent)

	t.status(StatusChecking, i18n.T("Establishing connection"))
	client, err := t.connect(ctx)
	if err != nil {
		cancel()
		t.status(StatusError, err.Error())
		return err
	}

	ln, err := net.Listen("tcp", t.cfg.LocalAddr())
	if err != nil {
		client.Close()
		cancel()
		e := fmt.Errorf(i18n.T("Could not bind local address %s: %w"), t.cfg.LocalAddr(), err)
		t.status(StatusError, e.Error())
		return e
	}

	t.mu.Lock()
	t.client = client
	t.listener = ln
	t.cancel = cancel
	t.conns = make(map[io.Closer]struct{})
	t.running = true
	t.mu.Unlock()

	t.total.Store(0)
	t.emitStats()

	go func() {
		<-ctx.Done()
		t.Stop()
	}()
	go t.acceptLoop(ctx, ln)
	go t.monitor(ctx)

	t.status(StatusActive, fmt.Sprintf(i18n.T("Tunnel active; connect clients to %s"), t.cfg.LocalAddr()))
	return nil
}

// Stop closes the listener, active client connections, and the SSH connection.
func (t *Tunnel) Stop() {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return
	}
	t.running = false
	cancel, ln, client := t.cancel, t.listener, t.client
	open := make([]io.Closer, 0, len(t.conns))
	for c := range t.conns {
		open = append(open, c)
	}
	t.cancel, t.listener, t.client, t.conns = nil, nil, nil, nil
	t.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range open {
		_ = c.Close()
	}
	if client != nil {
		_ = client.Close()
	}

	t.emitStats()
	t.status(StatusIdle, i18n.T("Tunnel stopped"))
}

// connect establishes a jump-server connection and verifies a channel to the destination.
// Each stage is logged separately to identify failures.
func (t *Tunnel) connect(ctx context.Context) (*ssh.Client, error) {
	auth, authResource, err := buildAuth(t.cfg.Auth)
	if err != nil {
		return nil, err
	}
	if authResource != nil {
		defer authResource.Close()
	}
	hostKey, err := newHostKeyCallback(t.cfg.VerifyHostKey, t.cfg.KnownHostsFile, t.cfg.HostKeyPrompt, t.logf)
	if err != nil {
		return nil, err
	}

	jumpAddr := t.cfg.JumpAddr()
	t.logf(i18n.T("1/3 TCP connection to jump server %s"), jumpAddr)
	dialer := net.Dialer{Timeout: t.cfg.ConnectTimeout}
	raw, err := dialer.DialContext(ctx, "tcp", jumpAddr)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("Jump server %s is unreachable via TCP: %w"), jumpAddr, err)
	}

	t.logf(i18n.T("2/3 SSH handshake and authentication for %s using %q"), t.cfg.JumpUser, t.cfg.Auth.Method)
	_ = raw.SetDeadline(time.Now().Add(t.cfg.ConnectTimeout))
	sshConn, chans, reqs, err := ssh.NewClientConn(raw, jumpAddr, &ssh.ClientConfig{
		User:            t.cfg.JumpUser,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         t.cfg.ConnectTimeout,
	})
	if err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf(i18n.T("SSH connection to jump server %s failed: %w"), jumpAddr, err)
	}
	_ = raw.SetDeadline(time.Time{})
	client := ssh.NewClient(sshConn, chans, reqs)

	targetAddr := t.cfg.TargetAddr()
	t.logf(i18n.T("3/3 checking channel to target %s through jump server"), targetAddr)
	probeCtx, cancel := context.WithTimeout(ctx, t.cfg.ConnectTimeout)
	defer cancel()
	probe, err := client.DialContext(probeCtx, "tcp", targetAddr)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf(i18n.T("Jump server did not open a channel to %s: %w"), targetAddr, err)
	}
	_ = probe.Close()

	t.logf(i18n.T("Connection established: %s is reachable through %s"), targetAddr, jumpAddr)
	return client, nil
}

// acceptLoop accepts local connections and handles each in a separate goroutine.
func (t *Tunnel) acceptLoop(ctx context.Context, ln net.Listener) {
	for {
		local, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			t.logf(i18n.T("Could not accept a local connection: %v"), err)
			continue
		}
		go t.handle(ctx, local)
	}
}

// handle opens a destination channel for one local connection and transfers data in both directions.
// Errors are logged within this goroutine and do not terminate the process.
func (t *Tunnel) handle(ctx context.Context, local net.Conn) {
	id := t.total.Add(1)
	t.track(local)
	t.emitStats()
	defer func() {
		t.untrack(local)
		_ = local.Close()
		t.emitStats()
	}()

	client := t.currentClient()
	if client == nil {
		t.logf(i18n.T("Connection %d was rejected: no connection to the jump server"), id)
		return
	}

	dialCtx, cancel := context.WithTimeout(ctx, t.cfg.ConnectTimeout)
	defer cancel()
	remote, err := client.DialContext(dialCtx, "tcp", t.cfg.TargetAddr())
	if err != nil {
		t.logf(i18n.T("Connection %d: jump server could not open a channel to %s: %v"), id, t.cfg.TargetAddr(), err)
		return
	}
	t.track(remote)
	defer func() {
		t.untrack(remote)
		_ = remote.Close()
	}()

	t.logf(i18n.T("Connection %d opened: %s -> %s"), id, local.RemoteAddr(), t.cfg.TargetAddr())
	up, down := pipe(local, remote)
	t.logf(i18n.T("Connection %d closed: sent %d bytes, received %d bytes"), id, up, down)
}

// pipe transfers data in both directions and returns the byte counts.
func pipe(local, remote net.Conn) (up, down int64) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		up, _ = io.Copy(remote, local)
		closeWrite(remote)
	}()
	go func() {
		defer wg.Done()
		down, _ = io.Copy(local, remote)
		closeWrite(local)
	}()
	wg.Wait()
	return up, down
}

// closeWrite closes only the write direction when supported, so the peer receives EOF.
func closeWrite(c net.Conn) {
	type closeWriter interface{ CloseWrite() error }
	if cw, ok := c.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

// monitor watches the SSH connection and sends keepalive requests while waiting for it to close.
func (t *Tunnel) monitor(ctx context.Context) {
	for {
		client := t.currentClient()
		if client == nil {
			return
		}

		dead := make(chan error, 1)
		go func() { dead <- client.Wait() }()

		ticker := time.NewTicker(t.cfg.KeepAlive)
		var reason error
	watch:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case err := <-dead:
				reason = err
				ticker.Stop()
				break watch
			case <-ticker.C:
				// An unsupported response still indicates a live connection; check only transport errors.
				if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
					reason = err
					ticker.Stop()
					break watch
				}
			}
		}

		if ctx.Err() != nil {
			return
		}

		t.logf(i18n.T("Connection to the jump server was lost: %v"), reason)
		_ = client.Close()
		t.setClient(nil)

		if !t.cfg.AutoReconnect {
			t.status(StatusError, i18n.T("Connection to the jump server was lost; tunnel stopped"))
			t.Stop()
			return
		}
		if !t.reconnect(ctx) {
			return
		}
	}
}

// reconnect restores the jump connection with an increasing delay between attempts.
// The local listener remains open while reconnecting.
func (t *Tunnel) reconnect(ctx context.Context) bool {
	delay := time.Second
	for attempt := 1; ; attempt++ {
		t.status(StatusReconnecting, fmt.Sprintf(i18n.T("Reconnecting, attempt %d"), attempt))
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}

		client, err := t.connect(ctx)
		if err == nil {
			t.setClient(client)
			t.status(StatusActive, fmt.Sprintf(i18n.T("Connection restored; tunnel is active on %s"), t.cfg.LocalAddr()))
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		t.logf(i18n.T("Attempt %d failed: %v"), attempt, err)

		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
}

// currentClient returns the current SSH connection, or nil when disconnected.
func (t *Tunnel) currentClient() *ssh.Client {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.client
}

// setClient replaces the current SSH connection.
func (t *Tunnel) setClient(c *ssh.Client) {
	t.mu.Lock()
	t.client = c
	t.mu.Unlock()
}

// track registers a connection so Stop can close it.
func (t *Tunnel) track(c io.Closer) {
	t.mu.Lock()
	if t.conns != nil {
		t.conns[c] = struct{}{}
	}
	t.mu.Unlock()
}

// untrack removes a closed connection from the tracked set.
func (t *Tunnel) untrack(c io.Closer) {
	t.mu.Lock()
	if t.conns != nil {
		delete(t.conns, c)
	}
	t.mu.Unlock()
}

// Stats returns the current connection counters.
func (t *Tunnel) Stats() Stats {
	return Stats{Active: t.countLocal(), Total: t.total.Load()}
}

// countLocal counts open local TCP connections, excluding their remote counterparts.
func (t *Tunnel) countLocal() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for c := range t.conns {
		if _, ok := c.(*net.TCPConn); ok {
			n++
		}
	}
	return n
}

// status reports a new state to the interface and writes the message to the log.
func (t *Tunnel) status(s Status, msg string) {
	if t.cb.OnStatus != nil {
		t.cb.OnStatus(s, msg)
	}
	t.logf("%s", msg)
}

// emitStats sends updated connection counters to the interface.
func (t *Tunnel) emitStats() {
	if t.cb.OnStats != nil {
		t.cb.OnStats(t.Stats())
	}
}

// logf writes a formatted line to the interface log.
func (t *Tunnel) logf(format string, args ...any) {
	if t.cb.OnLog != nil {
		t.cb.OnLog(fmt.Sprintf(format, args...))
	}
}
