// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// isolateSSH gives the test its own home with one fresh key and no agent, so the
// result does not depend on the ~/.ssh of the machine that runs the tests.
func isolateSSH(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SSH_AUTH_SOCK", "")
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "test")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "id_ed25519"), pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestQuote(t *testing.T) {
	cases := map[string]string{"plain": `'plain'`, "it's": `'it'\''s'`, "a b;rm -rf /": `'a b;rm -rf /'`, "": `''`}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestUploadIsAtomic(t *testing.T) {
	f := &Fake{}
	if err := Upload(context.Background(), f, "/opt/x/file.yml", []byte("hello\n"), "600"); err != nil {
		t.Fatal(err)
	}
	cmd := f.Calls[0]
	for _, want := range []string{"base64 -d", "chmod 600 '/opt/x/file.yml.tmp'", "mv -f '/opt/x/file.yml.tmp' '/opt/x/file.yml'"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command lacks %q:\n%s", want, cmd)
		}
	}
}

func TestFakeRules(t *testing.T) {
	f := (&Fake{}).On("boom", "", context.DeadlineExceeded).OnN("once", "x", nil, 1).On("once", "y", nil)
	if _, err := f.Run(context.Background(), "do boom"); err == nil {
		t.Fatal("want error")
	}
	a, _ := f.Run(context.Background(), "once")
	b, _ := f.Run(context.Background(), "once")
	if a != "x" || b != "y" {
		t.Fatalf("got %q %q", a, b)
	}
}

func TestRedactHidesUploadedContent(t *testing.T) {
	var seen []string
	f := &Fake{}
	l := &Logged{Runner: f, Log: func(cmd, _ string, _ error, _ time.Duration) { seen = append(seen, Redact(cmd)) }}
	if err := Upload(context.Background(), l, "/opt/muken/apps/a/env", []byte("APP_KEY=super-secret-value"), "600"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || strings.Contains(seen[0], "c3VwZXI") || !strings.Contains(seen[0], "<hidden>") {
		t.Fatalf("the payload must be hidden: %v", seen)
	}
	if !strings.Contains(seen[0], "/opt/muken/apps/a/env") {
		t.Fatalf("the path must stay visible: %v", seen)
	}
	if !strings.Contains(f.Calls[0], "base64 -d") || strings.Contains(f.Calls[0], "<hidden>") {
		t.Fatal("the server must still get the real command")
	}
}

type streamer struct{ Fake }

func (s *streamer) RunStdin(context.Context, string, io.Reader) error { return nil }

func TestLoggedPassesStreamsThrough(t *testing.T) {
	calls := 0
	log := func(string, string, error, time.Duration) { calls++ }
	if err := (&Logged{Runner: &streamer{}, Log: log}).RunStdin(context.Background(), "cat > f", strings.NewReader("x")); err != nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if err := (&Logged{Runner: &Fake{}, Log: log}).RunStdin(context.Background(), "cat > f", strings.NewReader("x")); err == nil {
		t.Fatal("a runner without streaming must fail clearly")
	}
}

// silentServer accepts connections and never sends the SSH banner.
func silentServer(t *testing.T) (string, int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()
	return "127.0.0.1", l.Addr().(*net.TCPAddr).Port
}

func TestDialTimesOutWhenServerIsSilent(t *testing.T) {
	isolateSSH(t)
	host, port := silentServer(t)
	start := time.Now()
	_, err := Dial(context.Background(), DialOptions{Host: host, Port: port, User: "root", AcceptNew: true, Timeout: 300 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want a timeout error, got %v", err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("Dial took %s", took)
	}
}

func TestDialStopsWhenContextIsCancelled(t *testing.T) {
	isolateSSH(t)
	host, port := silentServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	_, err := Dial(ctx, DialOptions{Host: host, Port: port, User: "root", AcceptNew: true, Timeout: 30 * time.Second})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
