package tunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// testUser and testPass are credentials for the embedded jump server.
const (
	testUser = "tester"
	testPass = "secret"
)

// newTestSigner creates a host key for the embedded SSH server.
func newTestSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("could not generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("could not create signer: %v", err)
	}
	return signer
}

// startEchoServer starts a TCP server that echoes all received data.
func startEchoServer(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start echo server: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(c)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// startJumpServer starts a minimal SSH server that authenticates with a password
// and handles direct-tcpip requests.
func startJumpServer(t *testing.T) (host string, port int) {
	t.Helper()
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == testUser && string(pass) == testPass {
				return nil, nil
			}
			return nil, errors.New("invalid password")
		},
	}
	cfg.AddHostKey(newTestSigner(t))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start jump server: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveJump(c, cfg)
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// serveJump handles one embedded jump-server connection, authenticating the client
// and opening requested direct-tcpip channels.
func serveJump(c net.Conn, cfg *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		c.Close()
		return
	}
	defer conn.Close()
	go ssh.DiscardRequests(reqs)

	for nc := range chans {
		if nc.ChannelType() != "direct-tcpip" {
			_ = nc.Reject(ssh.UnknownChannelType, "only direct-tcpip is supported")
			continue
		}
		var req struct {
			Host       string
			Port       uint32
			OriginHost string
			OriginPort uint32
		}
		if err := ssh.Unmarshal(nc.ExtraData(), &req); err != nil {
			_ = nc.Reject(ssh.ConnectionFailed, "invalid request")
			continue
		}
		target := net.JoinHostPort(req.Host, strconv.Itoa(int(req.Port)))
		remote, err := net.DialTimeout("tcp", target, 3*time.Second)
		if err != nil {
			_ = nc.Reject(ssh.ConnectionFailed, err.Error())
			continue
		}
		ch, chReqs, err := nc.Accept()
		if err != nil {
			remote.Close()
			continue
		}
		go ssh.DiscardRequests(chReqs)
		go func() {
			defer remote.Close()
			defer ch.Close()
			done := make(chan struct{})
			go func() { _, _ = io.Copy(ch, remote); close(done) }()
			_, _ = io.Copy(remote, ch)
			<-done
		}()
	}
}

// freePort finds an available local TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not find an available port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// testConfig creates tunnel settings for tests.
func testConfig(localPort int, jumpHost string, jumpPort int, targetHost string, targetPort int) Config {
	return Config{
		LocalPort:  localPort,
		JumpHost:   jumpHost,
		JumpPort:   jumpPort,
		JumpUser:   testUser,
		TargetHost: targetHost,
		TargetPort: targetPort,
		Auth: AuthConfig{
			Method:   AuthPassword,
			Password: testPass,
		},
		VerifyHostKey:  false,
		ConnectTimeout: 5 * time.Second,
		KeepAlive:      time.Second,
	}
}

// TestTunnelForwardsData verifies that data passes through the jump server to the destination and back.
func TestTunnelForwardsData(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)
	localPort := freePort(t)

	tun := New(testConfig(localPort, jumpHost, jumpPort, echoHost, echoPort), Callbacks{})
	if err := tun.Start(context.Background()); err != nil {
		t.Fatalf("could not start tunnel: %v", err)
	}
	defer tun.Stop()

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 3*time.Second)
	if err != nil {
		t.Fatalf("could not connect to the tunnel's local port: %v", err)
	}
	defer conn.Close()

	want := "connection test"
	if _, err := conn.Write([]byte(want)); err != nil {
		t.Fatalf("could not write to tunnel: %v", err)
	}
	buf := make([]byte, len(want))
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("could not read from tunnel: %v", err)
	}
	if string(buf) != want {
		t.Fatalf("received %q through tunnel, want %q", buf, want)
	}

	if got := tun.Stats().Total; got != 1 {
		t.Fatalf("connection count is %d, want 1", got)
	}
}

// TestStartDoesNotOpenPortWhenTargetUnreachable verifies startup fails without
// binding the local port when the destination is unreachable.
func TestStartDoesNotOpenPortWhenTargetUnreachable(t *testing.T) {
	jumpHost, jumpPort := startJumpServer(t)
	deadPort := freePort(t) // No service is listening on this port.
	localPort := freePort(t)

	tun := New(testConfig(localPort, jumpHost, jumpPort, "127.0.0.1", deadPort), Callbacks{})
	err := tun.Start(context.Background())
	if err == nil {
		tun.Stop()
		t.Fatal("startup succeeded although the destination is unreachable")
	}
	if !strings.Contains(err.Error(), "did not open a channel") {
		t.Fatalf("error does not indicate an unreachable destination: %v", err)
	}
	if tun.Running() {
		t.Fatal("tunnel is marked as running after validation failed")
	}

	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)))
	if err != nil {
		t.Fatalf("local port %d remains busy after failed startup: %v", localPort, err)
	}
	ln.Close()
}

// TestCheckDetectsWrongPassword verifies that authentication errors are caught during checking.
func TestCheckDetectsWrongPassword(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)

	cfg := testConfig(freePort(t), jumpHost, jumpPort, echoHost, echoPort)
	cfg.Auth.Password = "incorrect"
	tun := New(cfg, Callbacks{})

	err := tun.Check(context.Background())
	if err == nil {
		t.Fatal("connection check passed with an incorrect password")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Fatalf("error does not indicate an SSH connection failure: %v", err)
	}
}

// TestCheckDetectsUnreachableJump verifies that an unreachable jump server is reported explicitly.
func TestCheckDetectsUnreachableJump(t *testing.T) {
	cfg := testConfig(freePort(t), "127.0.0.1", freePort(t), "127.0.0.1", 22)
	tun := New(cfg, Callbacks{})

	err := tun.Check(context.Background())
	if err == nil {
		t.Fatal("connection check passed with an unreachable jump server")
	}
	if !strings.Contains(err.Error(), "is unreachable via TCP") {
		t.Fatalf("error does not indicate an unreachable jump server: %v", err)
	}
}

// TestCheckDetectsBusyLocalPort verifies that an occupied local port is reported explicitly.
func TestCheckDetectsBusyLocalPort(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)

	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not occupy a local port: %v", err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	tun := New(testConfig(busyPort, jumpHost, jumpPort, echoHost, echoPort), Callbacks{})
	err = tun.Check(context.Background())
	if err == nil {
		t.Fatal("connection check passed with an occupied local port")
	}
	if !strings.Contains(err.Error(), "is busy") {
		t.Fatalf("error does not indicate that the port is busy: %v", err)
	}
}

// TestValidateRejectsBadFields verifies that invalid parameters are rejected before network access.
func TestValidateRejectsBadFields(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"zero local port", Config{LocalPort: 0}, "Local port"},
		{"missing jump address", Config{LocalPort: 8080}, "Jump server"},
		{"missing username", Config{LocalPort: 8080, JumpHost: "h", JumpPort: 22}, "username"},
		{"missing target address", Config{LocalPort: 8080, JumpHost: "h", JumpPort: 22, JumpUser: "u"}, "Target server"},
		{
			"unknown authentication method",
			Config{LocalPort: 8080, JumpHost: "h", JumpPort: 22, JumpUser: "u", TargetHost: "t", TargetPort: 22},
			"authentication method",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.cfg.Validate()
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not contain %q", err, c.want)
			}
		})
	}
}

// TestStopClosesActiveConnections verifies that stopping the tunnel closes active connections.
func TestStopClosesActiveConnections(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)
	localPort := freePort(t)

	tun := New(testConfig(localPort, jumpHost, jumpPort, echoHost, echoPort), Callbacks{})
	if err := tun.Start(context.Background()); err != nil {
		t.Fatalf("could not start tunnel: %v", err)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 3*time.Second)
	if err != nil {
		t.Fatalf("could not connect to tunnel: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("x")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, 1)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("no echo response received: %v", err)
	}

	tun.Stop()

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("connection remained open after the tunnel stopped")
	}
	if got := tun.Stats().Active; got != 0 {
		t.Fatalf("active connection count after stopping is %d, want 0", got)
	}
}

// TestParentContextCancellationStopsTunnel verifies that parent-context cancellation closes
// the listener, SSH connection, and active client connections.
func TestParentContextCancellationStopsTunnel(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)
	jumpHost, jumpPort := startJumpServer(t)
	localPort := freePort(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tun := New(testConfig(localPort, jumpHost, jumpPort, echoHost, echoPort), Callbacks{})
	if err := tun.Start(ctx); err != nil {
		t.Fatalf("could not start tunnel: %v", err)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 3*time.Second)
	if err != nil {
		t.Fatalf("could not connect to tunnel: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("x")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, 1)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("no echo response received: %v", err)
	}

	cancel()
	deadline := time.Now().Add(5 * time.Second)
	var ln net.Listener
	for time.Now().Before(deadline) {
		ln, err = net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)))
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ln == nil {
		t.Fatalf("local port was not released after context cancellation: %v", err)
	}
	_ = ln.Close()

	deadline = time.Now().Add(5 * time.Second)
	for tun.Stats().Active != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if tun.Running() {
		t.Fatal("tunnel remained running after context cancellation")
	}
	if got := tun.Stats().Active; got != 0 {
		t.Fatalf("active connection count after cancellation is %d, want 0", got)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("client connection remained open after context cancellation")
	}
}

// TestKeyAuthReportsMissingFile verifies that a missing key file has a specific error.
func TestKeyAuthReportsMissingFile(t *testing.T) {
	_, _, err := buildAuth(AuthConfig{Method: AuthKey, KeyPath: "/path/that/does/not/exist"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestPasswordAuthRequiresPassword verifies that an empty password is rejected before connecting.
func TestPasswordAuthRequiresPassword(t *testing.T) {
	_, _, err := buildAuth(AuthConfig{Method: AuthPassword})
	if err == nil || !strings.Contains(err.Error(), "Password is not set") {
		t.Fatalf("unexpected result: %v", err)
	}
}

// TestHostKeyMismatchIsRejected verifies that a changed host key is rejected.
func TestHostKeyMismatchIsRejected(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/known_hosts"

	stored := newTestSigner(t)
	if err := ensureFile(file); err != nil {
		t.Fatalf("could not create file: %v", err)
	}
	if err := appendKnownHost(file, "127.0.0.1:2222", stored.PublicKey()); err != nil {
		t.Fatalf("could not write host key: %v", err)
	}

	cb, err := newHostKeyCallback(true, file, nil, func(string, ...any) {})
	if err != nil {
		t.Fatalf("could not create host-key verifier: %v", err)
	}

	other := newTestSigner(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}
	err = cb("127.0.0.1:2222", addr, other.PublicKey())
	if err == nil {
		t.Fatal("changed host key was accepted")
	}
	if !strings.Contains(err.Error(), "differs from the previously saved key") {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := cb("127.0.0.1:2222", addr, stored.PublicKey()); err != nil {
		t.Fatalf("saved host key was rejected: %v", err)
	}
}

// TestHostKeyUnknownRequiresConfirmation verifies that new host keys require confirmation and are saved.
func TestHostKeyUnknownRequiresConfirmation(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/known_hosts"
	signer := newTestSigner(t)
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}

	deny, err := newHostKeyCallback(true, file, func(string, string, string) bool { return false }, func(string, ...any) {})
	if err != nil {
		t.Fatalf("could not create host-key verifier: %v", err)
	}
	if err := deny("127.0.0.1:2222", addr, signer.PublicKey()); err == nil {
		t.Fatal("unconfirmed host key was accepted")
	}

	allow, err := newHostKeyCallback(true, file, func(string, string, string) bool { return true }, func(string, ...any) {})
	if err != nil {
		t.Fatalf("could not create host-key verifier: %v", err)
	}
	if err := allow("127.0.0.1:2222", addr, signer.PublicKey()); err != nil {
		t.Fatalf("confirmed host key was rejected: %v", err)
	}

	again, err := newHostKeyCallback(true, file, func(string, string, string) bool {
		t.Fatal("host key was not remembered; the operator was prompted again")
		return false
	}, func(string, ...any) {})
	if err != nil {
		t.Fatalf("could not create host-key verifier: %v", err)
	}
	if err := again("127.0.0.1:2222", addr, signer.PublicKey()); err != nil {
		t.Fatalf("remembered host key was rejected: %v", err)
	}
}

// TestReconnectAfterJumpLoss verifies that automatic reconnect restores a lost jump connection.
func TestReconnectAfterJumpLoss(t *testing.T) {
	echoHost, echoPort := startEchoServer(t)

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == testUser && string(pass) == testPass {
				return nil, nil
			}
			return nil, errors.New("invalid password")
		},
	}
	cfg.AddHostKey(newTestSigner(t))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start jump server: %v", err)
	}
	defer ln.Close()
	jumpAddr := ln.Addr().(*net.TCPAddr)

	// Keep accepted connections so the test can close them explicitly.
	accepted := make(chan net.Conn, 8)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- c
			go serveJump(c, cfg)
		}
	}()

	localPort := freePort(t)
	tcfg := testConfig(localPort, jumpAddr.IP.String(), jumpAddr.Port, echoHost, echoPort)
	tcfg.AutoReconnect = true
	tcfg.KeepAlive = 300 * time.Millisecond

	statuses := make(chan Status, 32)
	tun := New(tcfg, Callbacks{OnStatus: func(s Status, _ string) {
		select {
		case statuses <- s:
		default:
		}
	}})

	if err := tun.Start(context.Background()); err != nil {
		t.Fatalf("could not start tunnel: %v", err)
	}
	defer tun.Stop()

	// Close the connection established by the tunnel to the jump server.
	select {
	case c := <-accepted:
		c.Close()
	case <-time.After(3 * time.Second):
		t.Fatal("jump server did not receive a connection from the tunnel")
	}

	deadline := time.After(15 * time.Second)
	sawReconnecting := false
	for {
		select {
		case s := <-statuses:
			if s == StatusReconnecting {
				sawReconnecting = true
			}
			if s == StatusActive && sawReconnecting {
				goto restored
			}
		case <-deadline:
			t.Fatalf("tunnel did not reconnect: reconnecting state observed=%v", sawReconnecting)
		}
	}

restored:
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)), 3*time.Second)
	if err != nil {
		t.Fatalf("connection failed after recovery: %v", err)
	}
	defer conn.Close()
	msg := []byte("after reconnect")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	buf := make([]byte, len(msg))
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("no echo response received after recovery: %v", err)
	}
	if string(buf) != string(msg) {
		t.Fatalf("received %q, want %q", buf, msg)
	}
}
