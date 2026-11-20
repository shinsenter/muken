// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

var ansiCodes = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

func visibleLen(s string) int { return utf8.RuneCountInString(ansiCodes.ReplaceAllString(s, "")) }

// width is the line width for sections, rows and prompts. It follows the
// terminal, between 40 and 100 columns, and is 80 when output is not a terminal.
func (p *Printer) width() int {
	f, ok := p.Out.(*os.File)
	if !ok {
		return 80
	}
	w, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 80
	}
	return min(max(w, 40), 100)
}

func (p *Printer) Section(title string) {
	p.endStep(true)
	dots := strings.Repeat(".", max(2, p.width()-6-visibleLen(title)))
	fmt.Fprintf(p.Out, "\n  %s %s\n", p.Paint(Heading, title), p.Paint(Dim, dots))
}

func (p *Printer) Row(label, value string) {
	p.endStep(true)
	dots := strings.Repeat(".", max(2, p.width()-6-visibleLen(label)-visibleLen(value)))
	fmt.Fprintf(p.Out, "  %s %s %s\n", label, p.Paint(Dim, dots), value)
}

func (p *Printer) Check(label, value string) { p.Row(label, p.Paint(Accent, value)) }

func (p *Printer) List(items [][2]string) { p.ListWidth(0, items) }

// ListWidth is List with a minimum width for the name column. Several lists
// that use the same width line up with each other.
func (p *Printer) ListWidth(width int, items [][2]string) {
	p.endStep(true)
	longest := width
	for _, item := range items {
		longest = max(longest, visibleLen(item[0]))
	}
	for _, item := range items {
		pad := strings.Repeat(" ", longest-visibleLen(item[0]))
		fmt.Fprintf(p.Out, "  %s%s  %s\n", p.Paint(Accent, item[0]), pad, item[1])
	}
}
