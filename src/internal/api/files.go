// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxFile = 256 << 10

func dirOf(configPath string) string {
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return filepath.Dir(configPath)
	}
	return filepath.Dir(abs)
}

var secretLine = regexp.MustCompile(`(?im)^(\s*-?\s*[\w.-]*(key|token|secret|password|passwd)[\w.-]*\s*:\s*).+$`)

// redact hides the value of every line whose key looks like a secret.
func redact(text string) string { return secretLine.ReplaceAllString(text, "${1}***") }

func (s *server) config(w http.ResponseWriter, r *http.Request) {
	if !s.needConfig(w) {
		return
	}
	data, err := os.ReadFile(s.opt.ConfigPath)
	if err != nil {
		failErr(w, err)
		return
	}
	write(w, 200, map[string]string{"path": s.opt.ConfigPath, "yaml": redact(string(data))})
}

// hidden says whether a path in the working folder is never shown: the private keys and similar files.
func hidden(rel string) bool {
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)
	switch {
	case rel == ".muken/age.key", rel == ".muken/.ssh", strings.HasPrefix(rel, ".muken/.ssh/"):
		return true
	case strings.HasPrefix(base, ".env"), strings.HasSuffix(base, ".pem"), strings.HasSuffix(base, ".key"),
		strings.HasSuffix(base, ".secrets.age"), strings.HasPrefix(base, "id_"):
		return true
	}
	return false
}

type entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

// files lists a folder or returns a text file of the working folder. It is read only.
func (s *server) files(w http.ResponseWriter, r *http.Request) {
	rel := filepath.Clean("/" + r.URL.Query().Get("path"))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		rel = "."
	}
	if hidden(rel) {
		fail(w, 403, "forbidden", "this file is not shown", "private keys and .env files stay out of the web")
		return
	}
	root, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		fail(w, 404, "not_found", "no working folder", "")
		return
	}
	full, err := filepath.EvalSymlinks(filepath.Join(root, rel))
	if err != nil {
		fail(w, 404, "not_found", "no such path", "")
		return
	}
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		fail(w, 403, "forbidden", "the path leaves the working folder", "")
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		fail(w, 404, "not_found", "no such path", "")
		return
	}
	if info.IsDir() {
		items, err := os.ReadDir(full)
		if err != nil {
			failErr(w, err)
			return
		}
		out := []entry{}
		for _, item := range items {
			if hidden(filepath.Join(rel, item.Name())) {
				continue
			}
			fi, err := item.Info()
			if err != nil {
				continue
			}
			out = append(out, entry{item.Name(), item.IsDir(), fi.Size()})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Dir != out[j].Dir {
				return out[i].Dir
			}
			return out[i].Name < out[j].Name
		})
		write(w, 200, map[string]any{"path": filepath.ToSlash(rel), "entries": out})
		return
	}
	if !info.Mode().IsRegular() || info.Size() > maxFile {
		fail(w, 413, "too_large", "not a regular file, or over 256 KB", "")
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		failErr(w, err)
		return
	}
	if bytes.IndexByte(data, 0) >= 0 {
		fail(w, 415, "binary", "this file is not text", "")
		return
	}
	write(w, 200, map[string]string{"path": filepath.ToSlash(rel), "content": string(data)})
}
