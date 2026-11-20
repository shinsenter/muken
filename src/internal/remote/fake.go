// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"context"
	"strings"
	"sync"
)

// Fake is a Runner for tests. It records commands and answers from rules.
type Fake struct {
	mu    sync.Mutex
	Calls []string
	rules []rule
}

type rule struct {
	match string
	out   string
	err   error
	times int // 0 means always
	used  int
}

// On answers any command that contains match.
func (f *Fake) On(match, out string, err error) *Fake {
	f.rules = append(f.rules, rule{match: match, out: out, err: err})
	return f
}

// OnN is like On but applies only to the first n matches.
func (f *Fake) OnN(match, out string, err error, n int) *Fake {
	f.rules = append(f.rules, rule{match: match, out: out, err: err, times: n})
	return f
}

func (f *Fake) Run(_ context.Context, cmd string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, cmd)
	for i := range f.rules {
		current := &f.rules[i]
		if !strings.Contains(cmd, current.match) || (current.times > 0 && current.used >= current.times) {
			continue
		}
		current.used++
		if current.err != nil {
			return current.out, &CmdError{Cmd: cmd, Stderr: current.err.Error(), Err: current.err}
		}
		return current.out, nil
	}
	return "", nil
}

func (f *Fake) Close() error { return nil }

// Index returns the position of the first call that contains substr, or -1.
func (f *Fake) Index(substr string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, call := range f.Calls {
		if strings.Contains(call, substr) {
			return i
		}
	}
	return -1
}

// Count returns how many calls contain s.
func (f *Fake) Count(substr string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, call := range f.Calls {
		if strings.Contains(call, substr) {
			n++
		}
	}
	return n
}
