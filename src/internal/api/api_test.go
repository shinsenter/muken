// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shinsenter/muken/internal/ui"
)

type fakeBackend struct {
	mu      sync.Mutex
	release chan struct{} // a Run waits for this, when it is set
	calls   [][]string
	fail    error
}

func (f *fakeBackend) Run(ctx context.Context, out *Lines, args []string) error {
	f.mu.Lock()
	f.calls = append(f.calls, args)
	f.mu.Unlock()
	out.Write([]byte("start " + args[0] + "\nsecond"))
	if f.release != nil {
		<-f.release
	}
	out.Write([]byte(" line\n"))
	return f.fail
}

func (f *fakeBackend) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeBackend) call(i int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i < 0 {
		i += len(f.calls)
	}
	return strings.Join(f.calls[i], " ")
}

func (f *fakeBackend) Status(ctx context.Context) (any, error) {
	return map[string]any{"release": "r1", "maintenance": false}, nil
}

func setup(t *testing.T, b Backend) (http.Handler, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "muken.yaml")
	return New(Options{Backend: b, ConfigPath: cfg, Version: "v-test"}), dir
}

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("bad JSON %q: %v", rec.Body.String(), err)
	}
	return m
}

func TestAPINoConfigIsConflict(t *testing.T) {
	h, _ := setup(t, &fakeBackend{})
	if rec := call(h, "GET", "/v1/version", ""); rec.Code != 200 {
		t.Fatalf("version: %d", rec.Code)
	}
	for _, path := range []string{"/v1/status", "/v1/plan", "/v1/doctor", "/v1/config"} {
		rec := call(h, "GET", path, "")
		if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"no_config"`) {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := call(h, "POST", "/v1/deploy", ""); rec.Code != 409 {
		t.Errorf("deploy without config: %d", rec.Code)
	}
}

func TestAPIStatusAndText(t *testing.T) {
	b := &fakeBackend{}
	h, dir := setup(t, b)
	os.WriteFile(filepath.Join(dir, "muken.yaml"), []byte("app: x\n"), 0o600)
	if m := decode(t, call(h, "GET", "/v1/status", "")); m["release"] != "r1" {
		t.Errorf("status: %v", m)
	}
	call(h, "GET", "/v1/plan", "")
	call(h, "GET", "/v1/logs?service=web&tail=20", "")
	if got := b.call(0); got != "deploy --plan" {
		t.Errorf("plan ran %q", got)
	}
	if got := b.call(1); got != "logs -n 20 web" {
		t.Errorf("logs ran %q", got)
	}
	for _, path := range []string{"/v1/logs?service=a%3Bb", "/v1/logs?tail=0", "/v1/logs?tail=9999"} {
		if rec := call(h, "GET", path, ""); rec.Code != 400 {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

func TestAPIFailureCarriesTheHint(t *testing.T) {
	h, dir := setup(t, &fakeBackend{fail: ui.Errf("run muken up", "no server")})
	os.WriteFile(filepath.Join(dir, "muken.yaml"), []byte("app: x\n"), 0o600)
	rec := call(h, "GET", "/v1/doctor", "")
	m := decode(t, rec)
	e, _ := m["error"].(map[string]any)
	if rec.Code != 500 || e["hint"] != "run muken up" {
		t.Errorf("%d %v", rec.Code, m)
	}
}

func TestAPIJobsRunOneAtATime(t *testing.T) {
	b := &fakeBackend{release: make(chan struct{})}
	h, dir := setup(t, b)
	os.WriteFile(filepath.Join(dir, "muken.yaml"), []byte("app: x\n"), 0o600)

	rec := call(h, "POST", "/v1/deploy", `{"image":"example/app:1"}`)
	if rec.Code != 202 {
		t.Fatalf("deploy: %d %s", rec.Code, rec.Body)
	}
	id := decode(t, rec)["job_id"].(string)

	second := call(h, "POST", "/v1/rollback", "")
	m := decode(t, second)
	if second.Code != 409 || m["job_id"] != id {
		t.Errorf("second job: %d %v", second.Code, m)
	}

	close(b.release)
	var job map[string]any
	for i := 0; i < 100; i++ {
		job = decode(t, call(h, "GET", "/v1/jobs/"+id+"?after=1", ""))
		if job["state"] != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job["state"] != "ok" {
		t.Fatalf("state: %v", job)
	}
	lines := job["lines"].([]any)
	if len(lines) != 1 || lines[0] != "second line" || job["next"].(float64) != 2 {
		t.Errorf("lines after 1: %v next %v", lines, job["next"])
	}
	if got := b.call(0); got != "deploy --image example/app:1" {
		t.Errorf("deploy ran %q", got)
	}
	if rec := call(h, "POST", "/v1/rollback", ""); rec.Code != 202 {
		t.Errorf("a new job after the end: %d", rec.Code)
	}
	if rec := call(h, "GET", "/v1/jobs/nope", ""); rec.Code != 404 {
		t.Errorf("unknown job: %d", rec.Code)
	}
}

func TestAPIFailedJob(t *testing.T) {
	h, dir := setup(t, &fakeBackend{fail: errors.New("boom")})
	os.WriteFile(filepath.Join(dir, "muken.yaml"), []byte("app: x\n"), 0o600)
	id := decode(t, call(h, "POST", "/v1/up", ""))["job_id"].(string)
	var job map[string]any
	for i := 0; i < 100; i++ {
		job = decode(t, call(h, "GET", "/v1/jobs/"+id, ""))
		if job["state"] != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job["state"] != "failed" || job["error"] != "boom" {
		t.Errorf("%v", job)
	}
}

func TestAPIBodies(t *testing.T) {
	b := &fakeBackend{}
	h, dir := setup(t, b)
	os.WriteFile(filepath.Join(dir, "muken.yaml"), []byte("app: x\n"), 0o600)
	cases := []struct {
		path, body string
		code       int
		want       string
	}{
		{"/v1/scale", `{"scale":{"web":3,"queue":2}}`, 202, "scale queue=2 web=3"},
		{"/v1/maintenance", `{"on":true}`, 202, "maintenance on"},
		{"/v1/maintenance", `{"on":false}`, 202, "maintenance off"},
		{"/v1/maintenance", `{}`, 400, ""},
		{"/v1/scale", `{"scale":{"web":0}}`, 400, ""},
		{"/v1/deploy", `{"image":"bad image;rm"}`, 400, ""},
	}
	for _, c := range cases {
		before := b.count()
		rec := call(h, "POST", c.path, c.body)
		if rec.Code != c.code {
			t.Errorf("%s %s: %d", c.path, c.body, rec.Code)
		}
		if c.want != "" {
			for i := 0; i < 100 && b.count() == before; i++ {
				time.Sleep(10 * time.Millisecond)
			}
			if got := b.call(-1); got != c.want {
				t.Errorf("%s ran %q, want %q", c.path, got, c.want)
			}
			id := decode(t, rec)["job_id"].(string)
			for i := 0; i < 100; i++ { // wait, so the next case can start
				if decode(t, call(h, "GET", "/v1/jobs/"+id, ""))["state"] != "running" {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

func TestAPIConfigHidesSecrets(t *testing.T) {
	h, dir := setup(t, &fakeBackend{})
	yaml := "app: x\nenv:\n  API_TOKEN: abc123\n  MODE: prod\ncloudflare:\n  api_key: zzz\n"
	os.WriteFile(filepath.Join(dir, "muken.yaml"), []byte(yaml), 0o600)
	got := decode(t, call(h, "GET", "/v1/config", ""))["yaml"].(string)
	if strings.Contains(got, "abc123") || strings.Contains(got, "zzz") {
		t.Errorf("secret shown: %s", got)
	}
	if !strings.Contains(got, "MODE: prod") || !strings.Contains(got, "API_TOKEN: ***") {
		t.Errorf("redaction wrong: %s", got)
	}
}

func TestAPIFilesAreReadOnlyAndStayInside(t *testing.T) {
	h, dir := setup(t, &fakeBackend{})
	os.WriteFile(filepath.Join(dir, "muken.yaml"), []byte("app: x\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0o600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=1"), 0o600)
	os.WriteFile(filepath.Join(dir, "bin.dat"), []byte{1, 0, 2}, 0o600)
	os.MkdirAll(filepath.Join(dir, ".muken", ".ssh"), 0o700)
	os.WriteFile(filepath.Join(dir, ".muken", "age.key"), []byte("AGE-SECRET-KEY-1"), 0o600)
	os.WriteFile(filepath.Join(dir, ".muken", ".ssh", "id_ed25519"), []byte("key"), 0o600)
	outside := filepath.Join(filepath.Dir(dir), "outside.txt")
	os.WriteFile(outside, []byte("x"), 0o600)
	t.Cleanup(func() { os.Remove(outside) })
	os.Symlink(outside, filepath.Join(dir, "link.txt"))

	if m := decode(t, call(h, "GET", "/v1/files?path=notes.txt", "")); m["content"] != "hello" {
		t.Errorf("read: %v", m)
	}
	list := call(h, "GET", "/v1/files", "").Body.String()
	if !strings.Contains(list, "notes.txt") || strings.Contains(list, ".env") {
		t.Errorf("list: %s", list)
	}
	if strings.Contains(call(h, "GET", "/v1/files?path=.muken", "").Body.String(), "age.key") {
		t.Error("the age key is listed")
	}
	cases := map[string]int{
		".env":                   403,
		".muken/age.key":         403,
		".muken/.ssh/id_ed25519": 403,
		"../outside.txt":         404, // Clean("/..") keeps it inside, so the file does not exist
		"link.txt":               403,
		"bin.dat":                415,
		"missing":                404,
	}
	for path, want := range cases {
		if rec := call(h, "GET", "/v1/files?path="+path, ""); rec.Code != want {
			t.Errorf("%s: %d, want %d", path, rec.Code, want)
		}
	}
}

func TestCheckLoopback(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:7600": true, "localhost:7600": true, "[::1]:7600": true,
		"0.0.0.0:7600": false, ":7600": false, "10.0.0.5:7600": false, "nonsense": false,
	} {
		if err := CheckLoopback(addr); (err == nil) != ok {
			t.Errorf("%s: %v", addr, err)
		}
	}
}
