// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Lines collects the text of a command, one line at a time. It is safe for concurrent use.
type Lines struct {
	mu      sync.Mutex
	lines   []string
	partial string
}

func (l *Lines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	text := l.partial + string(p)
	parts := strings.Split(text, "\n")
	l.partial = parts[len(parts)-1]
	l.lines = append(l.lines, parts[:len(parts)-1]...)
	return len(p), nil
}

// From returns the lines after n and the new position.
func (l *Lines) From(n int) ([]string, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n < 0 || n > len(l.lines) {
		n = 0
	}
	return append([]string(nil), l.lines[n:]...), len(l.lines)
}

func (l *Lines) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	text := strings.Join(l.lines, "\n")
	if l.partial != "" {
		text += "\n" + l.partial
	}
	return text
}

// flush moves a last line without a newline into the list.
func (l *Lines) flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.partial != "" {
		l.lines = append(l.lines, l.partial)
		l.partial = ""
	}
}

// Job is one change that runs in the background.
type Job struct {
	ID     string
	Action string
	out    Lines

	mu    sync.Mutex
	state string // running, ok, failed
	err   string
	done  chan struct{}
}

func (j *Job) snapshot() (state, err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state, j.err
}

// start answers 202 and runs the command in the background. One change runs at a time.
func (s *server) start(action string, build func(*http.Request) ([]string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.needConfig(w) {
			return
		}
		args, err := build(r)
		if err != nil {
			fail(w, 400, "bad_request", err.Error(), "")
			return
		}
		s.mu.Lock()
		if s.running != "" {
			id := s.running
			s.mu.Unlock()
			write(w, 409, map[string]any{
				"error":  map[string]string{"code": "busy", "message": "another change is running", "hint": "wait for it to end"},
				"job_id": id,
			})
			return
		}
		s.seq++
		job := &Job{ID: fmt.Sprintf("j%d-%d", time.Now().Unix(), s.seq), Action: action, state: "running", done: make(chan struct{})}
		s.jobs[job.ID] = job
		s.order = append(s.order, job.ID)
		for len(s.order) > maxJobs {
			delete(s.jobs, s.order[0])
			s.order = s.order[1:]
		}
		s.running = job.ID
		s.mu.Unlock()

		go func() {
			err := s.opt.Backend.Run(s.opt.Context, &job.out, args)
			job.out.flush()
			job.mu.Lock()
			job.state = "ok"
			if err != nil {
				job.state, job.err = "failed", err.Error()
			}
			job.mu.Unlock()
			s.mu.Lock()
			s.running = ""
			s.mu.Unlock()
			close(job.done)
		}()
		write(w, 202, map[string]string{"job_id": job.ID})
	}
}

func (s *server) find(w http.ResponseWriter, r *http.Request) *Job {
	s.mu.Lock()
	job := s.jobs[r.PathValue("id")]
	s.mu.Unlock()
	if job == nil {
		fail(w, 404, "not_found", "no such job", "the server keeps only the last 50 jobs")
	}
	return job
}

func (s *server) job(w http.ResponseWriter, r *http.Request) {
	job := s.find(w, r)
	if job == nil {
		return
	}
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	state, errText := job.snapshot()
	lines, next := job.out.From(after)
	write(w, 200, map[string]any{"job_id": job.ID, "action": job.Action, "state": state, "error": errText, "lines": lines, "next": next})
}

// events streams the lines as server-sent events, then one "end" event.
func (s *server) events(w http.ResponseWriter, r *http.Request) {
	job := s.find(w, r)
	if job == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, 500, "failed", "streaming is not supported", "")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	pos := 0
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		pos, _ = strconv.Atoi(v)
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		var lines []string
		lines, pos = job.out.From(pos)
		for i, line := range lines {
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", pos-len(lines)+i+1, line)
		}
		select {
		case <-job.done:
			lines, pos = job.out.From(pos)
			for i, line := range lines {
				fmt.Fprintf(w, "id: %d\ndata: %s\n\n", pos-len(lines)+i+1, line)
			}
			state, errText := job.snapshot()
			fmt.Fprintf(w, "event: end\ndata: %s %s\n\n", state, errText)
			flusher.Flush()
			return
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
		flusher.Flush()
	}
}
