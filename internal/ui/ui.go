// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package ui prints progress and asks questions in the terminal.
package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// HintError carries a short fix suggestion next to the message.
type HintError struct {
	Msg  string
	Hint string
	Err  error
}

func (e *HintError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func (e *HintError) Unwrap() error { return e.Err }

func Errf(hint, format string, a ...any) error {
	return &HintError{Msg: fmt.Sprintf(format, a...), Hint: hint}
}

func Wrap(err error, hint, format string, a ...any) error {
	return &HintError{Msg: fmt.Sprintf(format, a...), Hint: hint, Err: err}
}

type Printer struct {
	Out         io.Writer
	Err         io.Writer
	In          *bufio.Reader
	Color       bool
	Interactive bool
	Yes         bool
	Verbose     bool // print each remote command and its output to stderr

	mu   sync.Mutex
	spin *spinner // the running step on a color terminal, nil elsewhere
}

func New() *Printer {
	return &Printer{
		Out:         os.Stdout,
		Err:         os.Stderr,
		In:          bufio.NewReader(os.Stdin),
		Color:       isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "",
		Interactive: isTerminal(os.Stdin) && isTerminal(os.Stdout),
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// Paint colors s with an ANSI code: 31 red, 32 green, 33 yellow, 36 cyan, 2 dim.
// It returns s as it is when the output is not a color terminal.
func (p *Printer) Paint(code, s string) string {
	if !p.Color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

// sym picks a Unicode symbol on a color terminal and plain text elsewhere.
func (p *Printer) sym(fancy, plain string) string {
	if p.Color {
		return fancy
	}
	return plain
}

// Step announces a step that is starting. On a color terminal it shows a
// spinner and the elapsed time, and the line is rewritten when the step ends.
// Elsewhere it prints one plain line, so logs and CI output stay stable.
func (p *Printer) Step(format string, a ...any) {
	title := fmt.Sprintf(format, a...)
	p.endStep(true)
	// A spinner would be overwritten by the verbose lines, so verbose uses plain lines.
	if !p.Color || p.Verbose {
		fmt.Fprintf(p.Out, "%s %s\n", p.Paint(Accent, p.sym("→", "->")), title)
		return
	}
	p.startSpinner(title)
}

func (p *Printer) OK(format string, a ...any) {
	p.endStep(true)
	fmt.Fprintf(p.Out, "%s %s\n", p.Paint(Accent, p.sym("✓", "ok")), fmt.Sprintf(format, a...))
}

func (p *Printer) Warn(format string, a ...any) {
	p.endStep(true)
	fmt.Fprintf(p.Err, "%s %s\n", p.Paint(Warning, "!"), fmt.Sprintf(format, a...))
}

func (p *Printer) Info(format string, a ...any) {
	p.endStep(true)
	fmt.Fprintf(p.Out, format+"\n", a...)
}

func (p *Printer) Fail(err error) {
	p.endStep(false)
	fmt.Fprintf(p.Err, "%s %s\n", p.Paint(Danger, p.sym("✗", "FAIL")), err.Error())
	var h *HintError
	if errors.As(err, &h) && h.Hint != "" {
		fmt.Fprintf(p.Err, "  %s %s\n", p.Paint(Warning, p.sym("→", "hint:")), h.Hint)
	}
}

// Debug prints a line to stderr, only in verbose mode.
func (p *Printer) Debug(format string, a ...any) {
	if !p.Verbose {
		return
	}
	p.endStep(true)
	fmt.Fprintln(p.Err, p.Paint(Dim, fmt.Sprintf(format, a...)))
}

// Command reports a remote command: the command, its time, a short tail of
// its output and the error if there is one. Verbose mode only.
func (p *Printer) Command(cmd, out string, err error, took time.Duration) {
	if !p.Verbose {
		return
	}
	const maxCmd, maxLines = 300, 12
	cmd = strings.Join(strings.Fields(cmd), " ")
	if len([]rune(cmd)) > maxCmd {
		cmd = string([]rune(cmd)[:maxCmd]) + " ..."
	}
	p.Debug("  $ %s  (%s)", cmd, elapsed(took))
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if strings.TrimSpace(out) == "" {
		lines = nil
	}
	if len(lines) > maxLines {
		p.Debug("  | ... %d earlier lines", len(lines)-maxLines)
		lines = lines[len(lines)-maxLines:]
	}
	for _, l := range lines {
		p.Debug("  | %s", l)
	}
	if err != nil {
		p.Debug("  ! %v", err)
	}
}

func (p *Printer) Finish(ok bool) { p.endStep(ok) }

// Table prints rows in aligned columns under a dim header. Rows with fewer
// cells than headers are padded.
func (p *Printer) Table(headers []string, rows [][]string) {
	p.endStep(true)
	width := make([]int, len(headers))
	for i, h := range headers {
		width[i] = len([]rune(h))
	}
	for _, r := range rows {
		for i := 0; i < len(headers) && i < len(r); i++ {
			if n := len([]rune(r[i])); n > width[i] {
				width[i] = n
			}
		}
	}
	line := func(cells []string, paint func(string) string) {
		var b strings.Builder
		for i := range headers {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			pad := strings.Repeat(" ", width[i]-len([]rune(cell)))
			if i == len(headers)-1 {
				pad = ""
			}
			b.WriteString(paint(cell) + pad)
			if i < len(headers)-1 {
				b.WriteString("  ")
			}
		}
		fmt.Fprintln(p.Out, b.String())
	}
	line(headers, func(s string) string { return p.Paint(Dim, s) })
	for _, r := range rows {
		line(r, func(s string) string { return s })
	}
}

func (p *Printer) Summary(title string, details [][2]string) {
	p.OK("%s", title)
	for _, detail := range details {
		p.Row(detail[0], detail[1])
	}
}

type spinner struct {
	title string
	start time.Time
	stop  chan struct{}
	done  chan struct{}
}

var redrawInterval = 100 * time.Millisecond

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (p *Printer) startSpinner(title string) {
	s := &spinner{title: title, start: time.Now(), stop: make(chan struct{}), done: make(chan struct{})}
	p.mu.Lock()
	p.spin = s
	p.mu.Unlock()
	draw := func(i int) {
		p.mu.Lock()
		defer p.mu.Unlock()
		fmt.Fprintf(p.Out, "\r\x1b[2K%s %s %s", p.Paint(Accent, spinnerFrames[i%len(spinnerFrames)]), s.title, p.Paint(Dim, elapsed(time.Since(s.start))))
	}
	draw(0)
	go func() {
		defer close(s.done)
		t := time.NewTicker(redrawInterval)
		defer t.Stop()
		for i := 1; ; i++ {
			select {
			case <-s.stop:
				return
			case <-t.C:
				draw(i)
			}
		}
	}()
}

func (p *Printer) endStep(ok bool) {
	p.mu.Lock()
	s := p.spin
	p.spin = nil
	p.mu.Unlock()
	if s == nil {
		return
	}
	close(s.stop)
	<-s.done
	mark := p.Paint(Accent, "✓")
	if !ok {
		mark = p.Paint(Danger, "✗")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.Out, "\r\x1b[2K%s %s %s\n", mark, s.title, p.Paint(Dim, elapsed(time.Since(s.start))))
}

// elapsed formats a duration for a step line: 800ms, 12s, 1m14s.
func elapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Round(time.Second).String()
}
