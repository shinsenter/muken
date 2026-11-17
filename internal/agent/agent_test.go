// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeDocker map[string]string

func (f fakeDocker) Get(_ context.Context, path string) ([]byte, error) {
	best := ""
	for prefix := range f {
		if strings.HasPrefix(path, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return nil, io.EOF
	}
	return []byte(f[best]), nil
}

func frame(stream byte, s string) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(s)))
	return append(header, s...)
}

func TestDemuxFramedAndPlain(t *testing.T) {
	framed := append(frame(1, "hello\nworld\n"), frame(2, "ERROR boom\n")...)
	got := demux(framed)
	if len(got) != 3 || got[0] != "hello" || got[2] != "ERROR boom" {
		t.Fatalf("framed: %v", got)
	}
	if got := demux([]byte("a\nb\n")); len(got) != 2 || got[1] != "b" {
		t.Fatalf("plain: %v", got)
	}
}

func TestParseStats(t *testing.T) {
	cpu, used, limit := parseStats([]byte(`{"cpu_stats":{"cpu_usage":{"total_usage":300},"system_cpu_usage":2000,"online_cpus":2},
	 "precpu_stats":{"cpu_usage":{"total_usage":100},"system_cpu_usage":1000},
	 "memory_stats":{"usage":1000,"limit":4000,"stats":{"inactive_file":200}}}`))
	if cpu != 40 || used != 800 || limit != 4000 {
		t.Fatalf("cpu=%v used=%v limit=%v", cpu, used, limit)
	}
}

func TestParseMeminfo(t *testing.T) {
	total, avail := parseMeminfo("MemTotal:        2048 kB\nMemFree: 1 kB\nMemAvailable:    1024 kB\n")
	if total != 2048*1024 || avail != 1024*1024 {
		t.Fatalf("%d %d", total, avail)
	}
}

func snapshotWith(containers ...ContainerInfo) Snapshot { return Snapshot{Containers: containers} }

func TestAlerterNeedsTwoPollsAndResolves(t *testing.T) {
	alerter := NewAlerter(2)
	down := snapshotWith(ContainerInfo{Name: "web-1", Role: "web", State: "exited", Current: true})
	if got := alerter.Evaluate(down); len(got) != 0 {
		t.Fatalf("alerter single bad poll must stay quiet: %v", got)
	}
	got := alerter.Evaluate(down)
	if len(got) != 1 || !strings.HasPrefix(got[0], "FIRING: web-1 is exited") {
		t.Fatalf("got %v", got)
	}
	if got := alerter.Evaluate(down); len(got) != 0 {
		t.Fatalf("must not repeat: %v", got)
	}
	up := snapshotWith(ContainerInfo{Name: "web-1", Role: "web", State: "running", Current: true, Ready: true})
	got = alerter.Evaluate(up)
	if len(got) != 1 || !strings.HasPrefix(got[0], "RESOLVED:") {
		t.Fatalf("got %v", got)
	}
	if len(alerter.Active()) != 0 {
		t.Fatal("nothing should be active")
	}
}

func TestOldReleasesNeverAlert(t *testing.T) {
	alerter := NewAlerter(1)
	old := snapshotWith(ContainerInfo{Name: "web-old", Role: "web", State: "exited", Current: false})
	if got := alerter.Evaluate(old); len(got) != 0 {
		t.Fatalf("alerter release kept for rollback is stopped on purpose: %v", got)
	}
}

func TestAlerterNotReadyMemoryDiskRestart(t *testing.T) {
	alerter := NewAlerter(1)
	snapshot := snapshotWith(ContainerInfo{Name: "w", Role: "web", State: "running", Current: true, Ready: false, MemUsed: 95, MemLimit: 100, Restarts: 0})
	snapshot.Host.DiskUsedRatio = 0.95
	got := strings.Join(alerter.Evaluate(snapshot), "\n")
	for _, want := range []string{"does not pass its health check", "95% of its memory limit", "disk is 95% full"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	snapshot.Containers[0].Restarts = 2
	if got := strings.Join(alerter.Evaluate(snapshot), "\n"); !strings.Contains(got, "RESTARTED: w restarted (2 total)") {
		t.Fatalf("got %s", got)
	}
}

func TestNoAlertWhenDockerUnreadable(t *testing.T) {
	alerter := NewAlerter(1)
	if got := alerter.Evaluate(Snapshot{Error: "cannot read Docker"}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestWebhookBody(t *testing.T) {
	var body string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer ts.Close()
	w := &Webhook{URL: ts.URL, Host: "vps1"}
	if err := w.Send(context.Background(), "FIRING: x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"text":"[vps1] FIRING: x"`) || !strings.Contains(body, `"content":"[vps1] FIRING: x"`) {
		t.Fatalf("body %s", body)
	}
	if err := (*Webhook)(nil).Send(context.Background(), "x"); err != nil {
		t.Fatal("nil webhook must be a no-op")
	}
}

func TestMetrics(t *testing.T) {
	out := Metrics(Snapshot{Host: HostInfo{Load1: 0.5}, Containers: []ContainerInfo{{Name: "w", App: "blog", Role: "web", State: "running", Ready: true, Current: true, MemUsed: 10, Restarts: 1}}})
	for _, want := range []string{"muken_host_load1 0.5", `muken_container_up{app="blog",role="web",name="w",current="true"} 1`, `muken_container_ready{app="blog",role="web",name="w",current="true"} 1`, `muken_container_restarts{app="blog",role="web",name="w",current="true"} 1`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestCollect(t *testing.T) {
	base := t.TempDir()
	os.MkdirAll(filepath.Join(base, "apps", "blog"), 0o755)
	os.WriteFile(filepath.Join(base, "apps", "blog", "state.json"), []byte(`{"release":"R2"}`), 0o644)
	os.WriteFile(filepath.Join(base, "apps", "blog", "history.log"), []byte("2026-11-16T10:00:00Z|blog|R2|img|ok\n"), 0o644)
	proc := t.TempDir()
	os.WriteFile(filepath.Join(proc, "loadavg"), []byte("0.75 0.5 0.3 1/100 999\n"), 0o644)
	os.WriteFile(filepath.Join(proc, "meminfo"), []byte("MemTotal: 1000 kB\nMemAvailable: 500 kB\n"), 0o644)

	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "ok") }))
	defer health.Close()
	port := health.URL[strings.LastIndex(health.URL, ":")+1:]

	docker := fakeDocker{
		"/containers/json": `[
		 {"Id":"a1","Names":["/web-new"],"State":"running","Labels":{"muken.app":"blog","muken.release":"R2","muken.role":"web","muken.health":"/readyz","muken.health_port":"` + port + `"}},
		 {"Id":"b2","Names":["/web-old"],"State":"exited","Labels":{"muken.app":"blog","muken.release":"R1","muken.role":"web"}},
		 {"Id":"c3","Names":["/muken-blog-postgres"],"State":"running","Labels":{"muken.app":"blog-accessory"}}]`,
		"/containers/a1/json":  `{"RestartCount":3,"State":{"StartedAt":"2026-11-16T09:00:00Z"}}`,
		"/containers/a1/stats": `{"memory_stats":{"usage":50,"limit":100}}`,
		"/containers/a1/logs":  string(append(frame(1, "GET / 200\n"), frame(2, "PHP Fatal error: x\n")...)),
	}
	// The probe goes to http://<name>:<port>; point the client at the test server instead.
	collector := &Collector{Docker: docker, ProcDir: proc, BaseDir: base, HTTP: &http.Client{Transport: redirect{health.URL}}}
	snapshot := collector.Collect(context.Background())
	if snapshot.Error != "" || len(snapshot.Containers) != 3 {
		t.Fatalf("snapshot: %+v", snapshot)
	}
	containersByName := map[string]ContainerInfo{}
	for _, container := range snapshot.Containers {
		containersByName[container.Name] = container
	}
	if n := containersByName["web-new"]; !n.Current || !n.Ready || n.Restarts != 3 || n.MemUsed != 50 {
		t.Fatalf("web-new: %+v", n)
	}
	if o := containersByName["web-old"]; o.Current {
		t.Fatalf("web-old must not be current: %+v", o)
	}
	if a := containersByName["muken-blog-postgres"]; a.Role != "accessory" || !a.Current {
		t.Fatalf("accessory: %+v", a)
	}
	if len(snapshot.Errors) != 1 || !strings.Contains(snapshot.Errors[0].Line, "Fatal error") {
		t.Fatalf("errors: %+v", snapshot.Errors)
	}
	if snapshot.Host.Load1 != 0.75 || snapshot.Host.MemAvailable != 500*1024 || len(snapshot.Deploys) != 1 {
		t.Fatalf("host/deploys: %+v %+v", snapshot.Host, snapshot.Deploys)
	}
}

type redirect struct{ to string }

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	u.Scheme, u.Host = "http", strings.TrimPrefix(r.to, "http://")
	req2 := req.Clone(req.Context())
	req2.URL = &u
	return http.DefaultTransport.RoundTrip(req2)
}

func TestCollectWhenDockerIsDown(t *testing.T) {
	s := (&Collector{Docker: fakeDocker{}, BaseDir: t.TempDir()}).Collect(context.Background())
	if s.Error == "" {
		t.Fatal("want an error message in the snapshot")
	}
}

func TestServerAuth(t *testing.T) {
	if _, err := New(Options{Addr: ":9100"}); err == nil {
		t.Fatal("a public address needs a token")
	}
	if _, err := New(Options{Addr: "127.0.0.1:9100"}); err != nil {
		t.Fatal("loopback without a token is fine:", err)
	}
	s, _ := New(Options{Addr: ":9100", Token: "secret", Collector: &Collector{Docker: fakeDocker{}, BaseDir: t.TempDir()}})
	h := s.Handler()
	do := func(path string, set func(*http.Request)) int {
		r := httptest.NewRequest("GET", path, nil)
		if set != nil {
			set(r)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := do("/api/status", nil); c != 401 {
		t.Fatalf("no credentials: %d", c)
	}
	if c := do("/api/status", func(r *http.Request) { r.Header.Set("Authorization", "Bearer wrong") }); c != 401 {
		t.Fatalf("wrong token: %d", c)
	}
	if c := do("/api/status", func(r *http.Request) { r.Header.Set("Authorization", "Bearer secret") }); c != 200 {
		t.Fatalf("bearer: %d", c)
	}
	if c := do("/metrics", func(r *http.Request) { r.SetBasicAuth("anyone", "secret") }); c != 200 {
		t.Fatalf("basic: %d", c)
	}
	if c := do("/healthz", nil); c != 200 {
		t.Fatalf("healthz must be open: %d", c)
	}
	if c := do("/nope", func(r *http.Request) { r.SetBasicAuth("x", "secret") }); c != 404 {
		t.Fatalf("unknown path: %d", c)
	}
}
