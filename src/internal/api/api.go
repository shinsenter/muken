// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package api is the JSON API that the web app calls. It has no login, so it must stay on loopback.
// It does not know how muken works: a Backend runs the commands, so a test can use a fake one.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/shinsenter/muken/internal/ui"
)

// Backend runs muken. Run executes one CLI command ("deploy", "--plan") and writes its text to out.
// Status returns the data that `muken status` shows.
type Backend interface {
	Run(ctx context.Context, out *Lines, args []string) error
	Status(ctx context.Context) (any, error)
	// Presets lists the images and apps that `muken init --preset` knows.
	Presets() any
}

type Options struct {
	Backend    Backend
	ConfigPath string // the fixed muken.yaml. Its folder is the working folder
	Version    string
	Context    context.Context // jobs stop when it ends. Default: never
}

type server struct {
	opt  Options
	root string

	mu      sync.Mutex
	jobs    map[string]*Job
	order   []string
	running string
	seq     int
}

const maxJobs = 50

// New returns the handler of the API.
func New(opt Options) http.Handler {
	if opt.Context == nil {
		opt.Context = context.Background()
	}
	s := &server{opt: opt, root: dirOf(opt.ConfigPath), jobs: map[string]*Job{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"version": opt.Version})
	})
	mux.HandleFunc("GET /v1/status", s.status)
	mux.HandleFunc("GET /v1/plan", s.text("deploy", "--plan"))
	mux.HandleFunc("GET /v1/doctor", s.text("doctor"))
	mux.HandleFunc("GET /v1/logs", s.logs)
	mux.HandleFunc("GET /v1/config", s.config)
	mux.HandleFunc("GET /v1/files", s.files)
	mux.HandleFunc("GET /v1/presets", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"presets": opt.Backend.Presets()})
	})
	mux.HandleFunc("POST /v1/init", s.init)
	mux.HandleFunc("GET /v1/server-key", s.keyShow)
	mux.HandleFunc("POST /v1/server-key", s.keyCreate)
	mux.HandleFunc("POST /v1/deploy", s.deploy)
	mux.HandleFunc("POST /v1/rollback", s.start("rollback", func(*http.Request) ([]string, error) { return []string{"rollback"}, nil }))
	mux.HandleFunc("POST /v1/up", s.start("up", func(*http.Request) ([]string, error) { return []string{"up"}, nil }))
	mux.HandleFunc("POST /v1/scale", s.start("scale", scaleArgs))
	mux.HandleFunc("POST /v1/maintenance", s.start("maintenance", maintenanceArgs))
	mux.HandleFunc("GET /v1/jobs/{id}", s.job)
	mux.HandleFunc("GET /v1/jobs/{id}/events", s.events)
	return mux
}

// CheckLoopback refuses an address that is not on loopback: the API has no login.
func CheckLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ui.Wrap(err, "use a form like 127.0.0.1:7600", "bad API address %q", addr)
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return nil
	}
	return ui.Errf("the API has no login, so it must stay on loopback, for example 127.0.0.1:7600", "refusing to listen on %q", addr)
}

func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, kind, msg, hint string) {
	write(w, code, map[string]any{"error": map[string]string{"code": kind, "message": msg, "hint": hint}})
}

// failErr answers with the hint that the CLI would print.
func failErr(w http.ResponseWriter, err error) {
	var he *ui.HintError
	if errors.As(err, &he) {
		fail(w, 500, "failed", err.Error(), he.Hint)
		return
	}
	fail(w, 500, "failed", err.Error(), "")
}

// fail422 answers a command that failed because of its input, with the text it printed.
func fail422(w http.ResponseWriter, err error, output string) {
	var he *ui.HintError
	hint := ""
	if errors.As(err, &he) {
		hint = he.Hint
	}
	write(w, 422, map[string]any{"error": map[string]string{"code": "failed", "message": err.Error(), "hint": hint}, "output": output})
}

// needConfig answers 409 when there is no muken.yaml.
func (s *server) needConfig(w http.ResponseWriter) bool {
	if _, err := os.Stat(s.opt.ConfigPath); err != nil {
		fail(w, 409, "no_config", "no muken.yaml at "+s.opt.ConfigPath, "open /setup in the web UI, or run `muken init` in the working folder, or mount the folder that has muken.yaml")
		return false
	}
	return true
}

func (s *server) status(w http.ResponseWriter, r *http.Request) {
	if !s.needConfig(w) {
		return
	}
	data, err := s.opt.Backend.Status(r.Context())
	if err != nil {
		failErr(w, err)
		return
	}
	write(w, 200, data)
}

// text runs a read-only command and answers with its text.
func (s *server) text(args ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { s.runText(w, r, args) }
}

func (s *server) runText(w http.ResponseWriter, r *http.Request, args []string) {
	if !s.needConfig(w) {
		return
	}
	var out Lines
	err := s.opt.Backend.Run(r.Context(), &out, args)
	if err != nil {
		var he *ui.HintError
		hint := ""
		if errors.As(err, &he) {
			hint = he.Hint
		}
		write(w, 500, map[string]any{"error": map[string]string{"code": "failed", "message": err.Error(), "hint": hint}, "output": out.String()})
		return
	}
	write(w, 200, map[string]string{"output": out.String()})
}

var serviceName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func (s *server) logs(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service == "" {
		service = "web"
	}
	tail := 100
	if v := r.URL.Query().Get("tail"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 2000 {
			fail(w, 400, "bad_request", "tail must be 1 to 2000", "")
			return
		}
		tail = n
	}
	if !serviceName.MatchString(service) {
		fail(w, 400, "bad_request", "bad service name", "use the role of a container, for example web")
		return
	}
	s.runText(w, r, []string{"logs", "-n", strconv.Itoa(tail), service})
}

var imageRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]{0,255}$`)

func (s *server) deploy(w http.ResponseWriter, r *http.Request) {
	s.start("deploy", func(r *http.Request) ([]string, error) {
		var body struct {
			Image string `json:"image"`
		}
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				return nil, fmt.Errorf("bad JSON body")
			}
		}
		args := []string{"deploy"}
		if body.Image != "" {
			if !imageRef.MatchString(body.Image) {
				return nil, fmt.Errorf("bad image name")
			}
			args = append(args, "--image", body.Image)
		}
		return args, nil
	})(w, r)
}

func scaleArgs(r *http.Request) ([]string, error) {
	var body struct {
		Scale map[string]int `json:"scale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Scale) == 0 {
		return nil, fmt.Errorf(`body must be {"scale": {"web": 3}}`)
	}
	names := make([]string, 0, len(body.Scale))
	for name, n := range body.Scale {
		if !serviceName.MatchString(name) || n < 1 || n > 100 {
			return nil, fmt.Errorf("bad scale %q=%d", name, n)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	args := []string{"scale"}
	for _, name := range names {
		args = append(args, fmt.Sprintf("%s=%d", name, body.Scale[name]))
	}
	return args, nil
}

func maintenanceArgs(r *http.Request) ([]string, error) {
	var body struct {
		On *bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.On == nil {
		return nil, fmt.Errorf(`body must be {"on": true}`)
	}
	if *body.On {
		return []string{"maintenance", "on"}, nil
	}
	return []string{"maintenance", "off"}, nil
}
