// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The setup endpoints make the first muken.yaml and an SSH key. They never overwrite a file:
// init stops when muken.yaml exists (no --force is ever passed), and a key is never replaced.

type initBody struct {
	Preset     string `json:"preset"`
	App        string `json:"app"`
	Host       string `json:"host"`
	Domain     string `json:"domain"`
	Email      string `json:"email"`
	Image      string `json:"image"`
	PHP        string `json:"php"`
	DB         string `json:"db"`
	Workers    string `json:"workers"`
	SSHKey     string `json:"ssh_key"` // the name of a key made by POST /v1/server-key
	Cloudflare bool   `json:"cloudflare"`
	Agent      bool   `json:"agent"`
}

var (
	presetName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	appName    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	hostName   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.:-]{0,253}$`)
	domainList = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.,-]{0,500}$`)
	emailAddr  = regexp.MustCompile(`^[A-Za-z0-9._%+][A-Za-z0-9._%+-]*@[A-Za-z0-9.-]+$`)
	phpVersion = regexp.MustCompile(`^[0-9]\.[0-9]{1,2}$`)
	dbList     = regexp.MustCompile(`^(postgres|mysql|mariadb|redis)(,(postgres|mysql|mariadb|redis))*$`)
	workerList = regexp.MustCompile(`^[a-z0-9][a-z0-9_,-]{0,200}$`)
	keyFile    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// args turns the body into muken init flags. Each flag is one argument, "--name=value", so a value
// can never be read as another flag. A bad field gives an error.
func (b initBody) args(keyPath string) ([]string, error) {
	out := []string{"init"}
	add := func(name, value string, ok *regexp.Regexp, required bool) error {
		if value == "" {
			if required {
				return fmt.Errorf("%s is required", name)
			}
			return nil
		}
		if !ok.MatchString(value) {
			return fmt.Errorf("bad %s", name)
		}
		out = append(out, "--"+name+"="+value)
		return nil
	}
	for _, f := range []struct {
		name, value string
		re          *regexp.Regexp
		required    bool
	}{
		{"preset", b.Preset, presetName, true},
		{"app", b.App, appName, true},
		{"host", b.Host, hostName, false},
		{"domain", b.Domain, domainList, false},
		{"email", b.Email, emailAddr, false},
		{"image", b.Image, imageRef, false},
		{"php", b.PHP, phpVersion, false},
		{"db", b.DB, dbList, false},
		{"workers", b.Workers, workerList, false},
	} {
		if err := add(f.name, f.value, f.re, f.required); err != nil {
			return nil, err
		}
	}
	if keyPath != "" {
		out = append(out, "--ssh-key="+keyPath)
	}
	if b.Cloudflare {
		out = append(out, "--cloudflare")
	}
	if b.Agent {
		out = append(out, "--agent")
	}
	return out, nil
}

func (s *server) keyDir() string { return filepath.Join(s.root, ".muken", ".ssh") }

// init makes muken.yaml with `muken init`. It answers 409 when the file exists.
func (s *server) init(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(s.opt.ConfigPath); err == nil {
		fail(w, 409, "config_exists", "muken.yaml already exists", "edit the file on the host. The web never overwrites it")
		return
	}
	var body initBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, 400, "bad_request", "bad JSON body", "")
		return
	}
	keyPath := ""
	if body.SSHKey != "" {
		if !keyFile.MatchString(body.SSHKey) {
			fail(w, 400, "bad_request", "bad ssh_key", "")
			return
		}
		keyPath = filepath.Join(s.keyDir(), body.SSHKey)
		if _, err := os.Stat(keyPath); err != nil {
			fail(w, 400, "bad_request", "no such key: create it first", "use POST /v1/server-key")
			return
		}
	}
	args, err := body.args(keyPath)
	if err != nil {
		fail(w, 400, "bad_request", err.Error(), "")
		return
	}
	var out Lines
	if err := s.opt.Backend.Run(r.Context(), &out, args); err != nil {
		fail422(w, err, out.String())
		return
	}
	write(w, 201, map[string]string{"output": out.String()})
}

func keyParam(r *http.Request) (string, bool) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "id_ed25519"
	}
	return name, keyFile.MatchString(name)
}

// keyCreate makes an SSH key with `muken server key <name>`. It never replaces a key.
func (s *server) keyCreate(w http.ResponseWriter, r *http.Request) {
	name, ok := keyParam(r)
	if !ok {
		fail(w, 400, "bad_request", "bad key name", "use letters, digits, '.', '-' and '_'")
		return
	}
	var out Lines
	if err := s.opt.Backend.Run(r.Context(), &out, []string{"server", "key", name}); err != nil {
		fail422(w, err, out.String())
		return
	}
	s.keyShow(w, r)
}

// keyShow gives the PUBLIC key only. The private key never leaves the folder.
func (s *server) keyShow(w http.ResponseWriter, r *http.Request) {
	name, ok := keyParam(r)
	if !ok {
		fail(w, 400, "bad_request", "bad key name", "")
		return
	}
	data, err := os.ReadFile(filepath.Join(s.keyDir(), name+".pub"))
	if err != nil {
		fail(w, 404, "not_found", "no such key", "make one with POST /v1/server-key")
		return
	}
	write(w, 200, map[string]string{"name": name, "path": filepath.Join(s.keyDir(), name), "public_key": strings.TrimSpace(string(data))})
}
