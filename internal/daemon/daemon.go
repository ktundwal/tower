package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
)

// Daemon wraps the HTTP server with a real listener, lockfile management,
// and clean shutdown.
type Daemon struct {
	server   *Server
	httpSrv  *http.Server
	listener net.Listener
	lockPath string
	port     int
	token    string
}

// Start launches a daemon on an ephemeral port, writes the lockfile, and
// begins accepting connections.
func Start(lockPath string) (*Daemon, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	srv := NewServer(token)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	info := LockInfo{
		Port:  port,
		Token: token,
		PID:   os.Getpid(),
	}
	if err := WriteLockfile(lockPath, info); err != nil {
		listener.Close()
		return nil, fmt.Errorf("write lockfile: %w", err)
	}

	httpSrv := &http.Server{Handler: srv}

	d := &Daemon{
		server:   srv,
		httpSrv:  httpSrv,
		listener: listener,
		lockPath: lockPath,
		port:     port,
		token:    token,
	}

	go httpSrv.Serve(listener)

	return d, nil
}

// Port returns the port the daemon is listening on.
func (d *Daemon) Port() int { return d.port }

// Token returns the auth token for this daemon instance.
func (d *Daemon) Token() string { return d.token }

// Server returns the underlying Server for session registration.
func (d *Daemon) Server() *Server { return d.server }

// Stop gracefully shuts down the HTTP server and removes the lockfile.
func (d *Daemon) Stop(ctx context.Context) error {
	err := d.httpSrv.Shutdown(ctx)
	_ = RemoveLockfile(d.lockPath)
	return err
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
