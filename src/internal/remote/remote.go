// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package remote runs commands on the server over SSH.
package remote

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/shinsenter/muken/internal/ui"
)

// Runner runs a shell command on the server and returns its standard output.
type Runner interface {
	Run(ctx context.Context, cmd string) (string, error)
	Close() error
}

type CmdError struct {
	Cmd    string
	Stderr string
	Err    error
}

func (e *CmdError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = e.Err.Error()
	}
	return msg
}

func (e *CmdError) Unwrap() error { return e.Err }

// Quote wraps s in single quotes for a POSIX shell.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Upload writes data to path with the given mode. The file appears in one step,
// so a reader such as Traefik never sees half of it.
func Upload(ctx context.Context, runner Runner, path string, data []byte, mode string) error {
	tmp := path + ".tmp"
	cmd := fmt.Sprintf("printf %%s %s | base64 -d > %s && chmod %s %s && mv -f %s %s",
		Quote(base64.StdEncoding.EncodeToString(data)), Quote(tmp), mode, Quote(tmp), Quote(tmp), Quote(path))
	_, err := runner.Run(ctx, cmd)
	return err
}

type DialOptions struct {
	Host      string
	Port      int
	User      string
	KeyPath   string
	AcceptNew bool
	Confirm   func(host, fingerprint string) bool
	Timeout   time.Duration
}

type SSH struct {
	client *ssh.Client
	sudo   bool
}

// Dial connects and checks the host key against ~/.ssh/known_hosts.
func Dial(ctx context.Context, opts DialOptions) (*SSH, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 15 * time.Second
	}
	auth, closers, err := authMethods(opts.KeyPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, closeAgent := range closers {
			closeAgent()
		}
	}()
	check, err := hostKeyCallback(opts)
	if err != nil {
		return nil, err
	}
	sshConfig := &ssh.ClientConfig{User: opts.User, Auth: auth, HostKeyCallback: check, Timeout: opts.Timeout}

	addr := net.JoinHostPort(opts.Host, fmt.Sprint(opts.Port))
	dialer := net.Dialer{Timeout: opts.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, ui.Wrap(err, "check server.host, server.port and the firewall", "cannot reach %s", addr)
	}
	sshConn, chans, reqs, err := handshake(ctx, conn, addr, sshConfig, opts.Timeout)
	if err != nil {
		conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, handshakeError(err, opts, addr)
	}
	return &SSH{client: ssh.NewClient(sshConn, chans, reqs), sudo: opts.User != "root"}, nil
}

// handshake runs the SSH handshake with a deadline and stops it when ctx ends.
// ssh.ClientConfig.Timeout does not cover this step, so a server that accepts
// the connection and then stays silent would block for ever.
func handshake(ctx context.Context, conn net.Conn, addr string, sshConfig *ssh.ClientConfig, timeout time.Duration) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	conn.SetDeadline(time.Now().Add(timeout))
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	conn.SetDeadline(time.Time{})
	return sshConn, chans, reqs, nil
}

func handshakeError(err error, opts DialOptions, addr string) error {
	var keyErr *knownhosts.KeyError
	if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
		return ui.Wrap(err, "the server key changed. If you rebuilt the server, remove the old line with: ssh-keygen -R "+opts.Host, "host key mismatch for %s", opts.Host)
	}
	if strings.Contains(err.Error(), "unable to authenticate") {
		return ui.Wrap(err, "load your key with ssh-add, or set server.ssh_key", "SSH login failed for %s@%s", opts.User, opts.Host)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ui.Wrap(err, "the server accepts the connection but does not answer SSH. Check that sshd runs on server.port", "SSH handshake with %s timed out", addr)
	}
	return ui.Wrap(err, "", "SSH connection to %s failed", addr)
}

// authMethods returns the ssh-agent and the key files that can log in, and the functions that close the agent.
func authMethods(keyPath string) ([]ssh.AuthMethod, []func(), error) {
	var methods []ssh.AuthMethod
	var closers []func()
	if method, closeAgent := agentAuth(); method != nil {
		methods = append(methods, method)
		closers = append(closers, closeAgent)
	}
	signers, err := keySigners(keyPath)
	if err != nil {
		return nil, nil, err
	}
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	if len(methods) == 0 {
		return nil, nil, ui.Errf("create a key with `ssh-keygen -t ed25519` or start ssh-agent", "no usable SSH key found")
	}
	return methods, closers, nil
}

// agentAuth returns nil when no ssh-agent is running.
func agentAuth() (ssh.AuthMethod, func()) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil
	}
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), func() { conn.Close() }
}

// keySigners reads the configured key, or tries the usual files in ~/.ssh.
// A missing configured key is an error. A missing default key is skipped.
func keySigners(keyPath string) ([]ssh.Signer, error) {
	home, _ := os.UserHomeDir()
	var paths []string
	if keyPath != "" {
		paths = []string{expandHome(keyPath, home)}
	} else {
		for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
			paths = append(paths, filepath.Join(home, ".ssh", name))
		}
	}
	var signers []ssh.Signer
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if keyPath != "" {
				return nil, ui.Wrap(err, "fix server.ssh_key in muken.yaml", "cannot read SSH key %s", path)
			}
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			continue // a key with a passphrase works through ssh-agent
		}
		signers = append(signers, signer)
	}
	return signers, nil
}

func expandHome(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// hostKeyCallback accepts hosts that are in known_hosts. A new host is added
// after the user confirms it, or at once with AcceptNew. A changed key always fails.
func hostKeyCallback(opts DialOptions) (ssh.HostKeyCallback, error) {
	path, err := knownHostsFile()
	if err != nil {
		return nil, err
	}
	check, err := knownhosts.New(path)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := check(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		if err == nil || !errors.As(err, &keyErr) || len(keyErr.Want) > 0 {
			return err
		}
		fingerprint := ssh.FingerprintSHA256(key)
		if !opts.AcceptNew && (opts.Confirm == nil || !opts.Confirm(hostname, fingerprint)) {
			return ui.Errf("check the fingerprint, then run again with --accept-new", "unknown host %s (%s)", hostname, fingerprint)
		}
		mu.Lock()
		defer mu.Unlock()
		return appendKnownHost(path, hostname, key)
	}, nil
}

// knownHostsFile returns ~/.ssh/known_hosts and creates it when missing.
func knownHostsFile() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".ssh")
	path := filepath.Join(dir, "known_hosts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600); err == nil {
		f.Close()
	}
	return path, nil
}

func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, knownhosts.Line([]string{hostname}, key))
	return err
}

// Run executes cmd. A non-root user runs it through `sudo -n`.
func (s *SSH) Run(ctx context.Context, cmd string) (string, error) {
	return s.exec(ctx, cmd, nil)
}

func (s *SSH) Close() error { return s.client.Close() }

// Streamer is a Runner that can also send a large file through stdin.
type Streamer interface {
	RunStdin(ctx context.Context, cmd string, in io.Reader) error
}

// RunStdin runs cmd and feeds it from in. It is used for the agent binary.
func (s *SSH) RunStdin(ctx context.Context, cmd string, in io.Reader) error {
	_, err := s.exec(ctx, cmd, in)
	return err
}

// exec runs cmd in a new session. It stops waiting when ctx ends.
func (s *SSH) exec(ctx context.Context, cmd string, stdin io.Reader) (string, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	var stdout, stderr bytes.Buffer
	sess.Stdin, sess.Stdout, sess.Stderr = stdin, &stdout, &stderr

	full := "sh -c " + Quote(cmd)
	if s.sudo {
		full = "sudo -n " + full
	}
	done := make(chan error, 1)
	go func() { done <- sess.Run(full) }()
	select {
	case <-ctx.Done():
		sess.Close()
		return stdout.String(), ctx.Err()
	case err := <-done:
		if err != nil {
			return stdout.String(), &CmdError{Cmd: cmd, Stderr: stderr.String(), Err: err}
		}
		return stdout.String(), nil
	}
}

var uploadPayload = regexp.MustCompile(`printf %s '[A-Za-z0-9+/=]*' \| base64 -d`)

// Redact hides the file contents that Upload sends inside a command. They can
// hold secrets, so verbose output must not show them.
func Redact(cmd string) string {
	return uploadPayload.ReplaceAllString(cmd, "printf %s '<hidden>' | base64 -d")
}

type Logged struct {
	Runner
	Log func(cmd, out string, err error, took time.Duration)
}

func (l *Logged) Run(ctx context.Context, cmd string) (string, error) {
	start := time.Now()
	out, err := l.Runner.Run(ctx, cmd)
	l.Log(cmd, out, err, time.Since(start))
	return out, err
}

func (l *Logged) RunStdin(ctx context.Context, cmd string, in io.Reader) error {
	streamer, ok := l.Runner.(Streamer)
	if !ok {
		return errors.New("this connection cannot stream a file")
	}
	start := time.Now()
	err := streamer.RunStdin(ctx, cmd, in)
	l.Log(cmd+" < (stream)", "", err, time.Since(start))
	return err
}
