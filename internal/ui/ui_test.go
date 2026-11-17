// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bufio"
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"
)

var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

// text drops the color and cursor codes, so a test reads what the user reads.
func withoutCodes(s string) string { return ansiPattern.ReplaceAllString(s, "") }

func newPlainPrinter() (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errOut bytes.Buffer
	return &Printer{Out: &out, Err: &errOut}, &out, &errOut
}

func TestPlainOutputHasNoEscapeCodesOrSpinner(t *testing.T) {
	p, out, errOut := newPlainPrinter()
	p.Step("Pulling image")
	p.OK("Done")
	p.Fail(Errf("run it again", "failed"))
	if got := out.String(); got != "-> Pulling image\nok Done\n" {
		t.Fatalf("stdout changed: %q", got)
	}
	if got := errOut.String(); got != "FAIL failed\n  hint: run it again\n" {
		t.Fatalf("stderr changed: %q", got)
	}
	if strings.Contains(out.String()+errOut.String(), "\x1b") {
		t.Fatal("no escape codes without a color terminal")
	}
}

func TestColorStepIsRewrittenWithTheResult(t *testing.T) {
	redrawInterval = 5 * time.Millisecond
	var out, errOut bytes.Buffer
	p := &Printer{Out: &out, Err: &errOut, Color: true}
	p.Step("Pulling image")
	time.Sleep(30 * time.Millisecond)
	p.OK("Done")
	got := withoutCodes(out.String())
	if !strings.Contains(got, "⠋ Pulling image") {
		t.Fatalf("a spinner frame is expected: %q", got)
	}
	if !strings.Contains(got, "✓") || !strings.Contains(got, "Pulling image") || !strings.Contains(got, "ms") {
		t.Fatalf("the finished step must show a check mark and a time: %q", got)
	}
	if p.spin != nil {
		t.Fatal("the spinner must be gone after the step ends")
	}
}

func TestFailMarksTheRunningStepAsFailed(t *testing.T) {
	redrawInterval = 5 * time.Millisecond
	var out, errOut bytes.Buffer
	p := &Printer{Out: &out, Err: &errOut, Color: true}
	p.Step("Running migrations")
	p.Fail(errors.New("boom"))
	if !strings.Contains(withoutCodes(out.String()), "✗ Running migrations") {
		t.Fatalf("the step must end with a cross: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "boom") {
		t.Fatalf("the error must be printed: %q", errOut.String())
	}
}

func TestFinishClosesAStepAtTheEndOfACommand(t *testing.T) {
	redrawInterval = 5 * time.Millisecond
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Color: true}
	p.Step("Starting the proxy")
	p.Finish(true)
	if !strings.Contains(withoutCodes(out.String()), "✓ Starting the proxy") || p.spin != nil {
		t.Fatalf("Finish must end the step: %q", out.String())
	}
	p.Finish(true) // calling it with no step is harmless
}

func TestTableAlignsColumns(t *testing.T) {
	p, out, _ := newPlainPrinter()
	p.Table([]string{"NAME", "STATE"}, [][]string{{"web-1", "running"}, {"mariadb", "exited"}, {"short"}})
	want := "NAME     STATE\nweb-1    running\nmariadb  exited\nshort    \n"
	if out.String() != want {
		t.Fatalf("got:\n%q\nwant:\n%q", out.String(), want)
	}
}

func TestSummaryUsesDottedRows(t *testing.T) {
	p, out, _ := newPlainPrinter()
	p.Summary("Release r1 is live", [][2]string{{"Image", "app:1"}, {"Open", "http://x"}})
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 || lines[0] != "ok Release r1 is live" {
		t.Fatalf("unexpected output:\n%s", out.String())
	}
	if !strings.HasPrefix(lines[1], "  Image ..") || !strings.HasSuffix(lines[1], ".. app:1") {
		t.Errorf("label left, value right: %q", lines[1])
	}
	if len(lines[1]) != 78 || len(lines[2]) != 78 {
		t.Errorf("rows must fill the line width (80 columns minus the margin): %d, %d", len(lines[1]), len(lines[2]))
	}
}

func TestSectionAndRow(t *testing.T) {
	p, out, _ := newPlainPrinter()
	p.Section("Environment")
	p.Row("PHP", "8.4")
	got := strings.Split(out.String(), "\n")
	if got[0] != "" || !strings.HasPrefix(got[1], "  Environment ..") {
		t.Fatalf("section: %q", got)
	}
	if !strings.HasPrefix(got[2], "  PHP ..") || !strings.HasSuffix(got[2], ".. 8.4") {
		t.Fatalf("row: %q", got[2])
	}
}

func TestRowCountsVisibleCharactersOnly(t *testing.T) {
	var a, b bytes.Buffer
	(&Printer{Out: &a, Err: &b}).Row("State", "OK")
	var c bytes.Buffer
	colored := &Printer{Out: &c, Err: &b, Color: true}
	colored.Row("State", colored.Paint(Accent, "OK"))
	if withoutCodes(c.String()) != a.String() {
		t.Fatalf("color codes must not change the layout:\n%q\n%q", withoutCodes(c.String()), a.String())
	}
}

func TestBoxPutsShortQuestionOnTheTopEdge(t *testing.T) {
	p, _, _ := newPlainPrinter()
	lines := p.box("Your name?", []string{"Jess"}, true)
	if len(lines) != 4 || !strings.HasPrefix(lines[1], "  ┌ Your name? ─") || !strings.HasSuffix(lines[1], "┐") {
		t.Fatalf("top edge: %q", lines)
	}
	if !strings.HasPrefix(lines[2], "  │ Jess") || !strings.HasSuffix(lines[2], " │") || !strings.HasPrefix(lines[3], "  └") {
		t.Fatalf("body and bottom: %q", lines)
	}
	if visibleLen(lines[1]) != visibleLen(lines[2]) || visibleLen(lines[2]) != visibleLen(lines[3]) {
		t.Fatalf("all lines must be the same width: %q", lines)
	}
}

func TestBoxWrapsALongQuestionAboveTheFrame(t *testing.T) {
	p, _, _ := newPlainPrinter()
	long := strings.Repeat("Create the server and pay for it. ", 4)
	lines := p.box(long, []string{"No"}, true)
	if !strings.Contains(strings.Join(lines, "\n"), "pay for it.") || strings.Contains(lines[len(lines)-3], "Create") {
		t.Fatalf("the whole question must be shown: %q", lines)
	}
	if !strings.HasPrefix(lines[len(lines)-3], "  ┌─") {
		t.Fatalf("the frame has no title on the edge: %q", lines)
	}
}

func TestReadKey(t *testing.T) {
	cases := []struct {
		in   string
		want key
	}{{"\r", keyEnter}, {"\x7f", keyBackspace}, {"\x03", keyCancel}, {" ", keySpace}, {"\x1b[A", keyUp}, {"\x1b[B", keyDown}, {"\x1b[C", keyRight}, {"\x1b[D", keyLeft}, {"x", keyRune}}
	for _, c := range cases {
		p := &Printer{In: bufio.NewReader(strings.NewReader(c.in))}
		if got, _, err := p.readKey(); err != nil || got != c.want {
			t.Errorf("readKey(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
}

func TestPromptsReturnDefaultsWithoutATerminal(t *testing.T) {
	p, out, _ := newPlainPrinter()
	if p.Ask("Name?", "app") != "app" || p.Choose("Pick", []string{"a", "b"}, 1) != 1 || p.Confirm("Sure?", true) != true || p.Multi("Many", []string{"a"}) != nil {
		t.Fatal("defaults expected")
	}
	if out.Len() != 0 {
		t.Fatalf("nothing is printed without a terminal: %q", out.String())
	}
	p.Yes = true
	if !p.Confirm("Sure?", false) {
		t.Fatal("--yes answers yes")
	}
}

func TestElapsed(t *testing.T) {
	cases := map[time.Duration]string{250 * time.Millisecond: "250ms", 12 * time.Second: "12s", 74 * time.Second: "1m14s"}
	for d, want := range cases {
		if got := elapsed(d); got != want {
			t.Errorf("elapsed(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestCommandIsSilentUnlessVerbose(t *testing.T) {
	p, out, errOut := newPlainPrinter()
	p.Command("docker ps", "web", nil, time.Second)
	p.Debug("hello")
	if out.String()+errOut.String() != "" {
		t.Fatalf("nothing expected without verbose: %q %q", out.String(), errOut.String())
	}
}

func TestVerboseCommandShowsTimeTailAndError(t *testing.T) {
	p, out, errOut := newPlainPrinter()
	p.Verbose = true
	var b strings.Builder
	for i := 1; i <= 20; i++ {
		b.WriteString("line " + strings.Repeat("x", 1) + "\n")
	}
	p.Command("docker   logs\n web", b.String(), errors.New("exit 1"), 1500*time.Millisecond)
	got := errOut.String()
	for _, want := range []string{"$ docker logs web  (2s)", "... 8 earlier lines", "| line x", "! exit 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if out.String() != "" {
		t.Fatal("verbose lines belong on stderr")
	}
}

func TestVerboseStepIsPlainEvenOnAColorTerminal(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, Err: &bytes.Buffer{}, Color: true, Verbose: true}
	p.Step("Pulling image")
	if p.spin != nil || !strings.Contains(withoutCodes(out.String()), "→ Pulling image\n") {
		t.Fatalf("a spinner would overwrite the verbose lines: %q", out.String())
	}
}
