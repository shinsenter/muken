// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

type key int

const (
	keyRune key = iota
	keyEnter
	keyBackspace
	keyUp
	keyDown
	keyLeft
	keyRight
	keySpace
	keyCancel
)

// readKey reads one key press. A terminal in raw mode sends an arrow key as
// the three bytes ESC [ A, and they arrive together, so they are in the buffer.
func (p *Printer) readKey() (key, rune, error) {
	r, _, err := p.In.ReadRune()
	if err != nil {
		return keyRune, 0, err
	}
	switch r {
	case '\r', '\n':
		return keyEnter, 0, nil
	case 0x7f, 0x08:
		return keyBackspace, 0, nil
	case 0x03, 0x04:
		return keyCancel, 0, nil
	case ' ':
		return keySpace, ' ', nil
	case 0x1b:
		if p.In.Buffered() < 2 {
			return keyRune, 0, nil
		}
		p.In.ReadRune() // the [ or O
		c, _, _ := p.In.ReadRune()
		switch c {
		case 'A':
			return keyUp, 0, nil
		case 'B':
			return keyDown, 0, nil
		case 'C':
			return keyRight, 0, nil
		case 'D':
			return keyLeft, 0, nil
		}
		return keyRune, 0, nil
	}
	if r < ' ' {
		return keyRune, 0, nil
	}
	return keyRune, r, nil
}

// enterRawMode puts the terminal in raw mode. The returned function restores it.
func (p *Printer) enterRawMode() (func(), bool) {
	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, false
	}
	fmt.Fprint(p.Out, "\x1b[?25l")
	return func() {
		fmt.Fprint(p.Out, "\x1b[?25h")
		term.Restore(fd, state)
	}, true
}

func (p *Printer) cancelAndExit(restore func()) {
	fmt.Fprint(p.Out, "\r\n")
	restore()
	p.Warn("cancelled")
	os.Exit(130)
}

type region struct {
	out io.Writer
	up  int
}

func (r *region) draw(lines []string) {
	if r.up > 0 {
		fmt.Fprintf(r.out, "\x1b[%dA", r.up)
	}
	fmt.Fprint(r.out, "\r\x1b[J", strings.Join(lines, "\r\n"))
	r.up = len(lines) - 1
}

// box draws a question with a frame. A short question sits on the top edge.
// A long one is printed above the frame, wrapped, so none of it is cut.
func (p *Printer) box(title string, body []string, active bool) []string {
	inner := min(p.width()-4, 76)
	for _, l := range body {
		inner = max(inner, min(visibleLen(l)+4, p.width()-4))
	}
	color := Dim
	if active {
		color = Accent
	}
	var lines []string
	lines = append(lines, "")
	top := p.Paint(Dim, "┌"+strings.Repeat("─", inner-2)+"┐")
	if visibleLen(title) <= inner-6 {
		dashes := strings.Repeat("─", inner-4-visibleLen(title))
		top = p.Paint(Dim, "┌ ") + p.Paint(color, title) + p.Paint(Dim, " "+dashes+"┐")
	} else {
		for _, l := range wrap(title, inner) {
			lines = append(lines, "  "+p.Paint(color, l))
		}
	}
	lines = append(lines, "  "+top)
	for _, l := range body {
		pad := strings.Repeat(" ", max(0, inner-4-visibleLen(l)))
		lines = append(lines, "  "+p.Paint(Dim, "│")+" "+l+pad+" "+p.Paint(Dim, "│"))
	}
	lines = append(lines, "  "+p.Paint(Dim, "└"+strings.Repeat("─", inner-2)+"┘"))
	return lines
}

func wrap(s string, width int) []string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && visibleLen(line)+1+visibleLen(w) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	return append(lines, line)
}

func (p *Printer) finish(r *region, restore func(), title, answer string) {
	r.draw(p.box(title, []string{answer}, false))
	fmt.Fprint(p.Out, "\r\n")
	restore()
}

func (p *Printer) cursor() string {
	if p.Color {
		return "\x1b[7m \x1b[0m"
	}
	return "_"
}

// Ask returns the typed answer, or defaultValue when the input is empty or not a terminal.
func (p *Printer) Ask(label, defaultValue string) string {
	if !p.Interactive {
		return defaultValue
	}
	restore, ok := p.enterRawMode()
	if !ok {
		return defaultValue
	}
	var typed []rune
	r := &region{out: p.Out}
	for {
		body := string(typed) + p.cursor()
		if len(typed) == 0 && defaultValue != "" {
			body = p.cursor() + p.Paint(Dim, defaultValue)
		}
		r.draw(p.box(label, []string{body}, true))
		k, c, err := p.readKey()
		switch {
		case err != nil:
			restore()
			return defaultValue
		case k == keyCancel:
			p.cancelAndExit(restore)
		case k == keyEnter:
			answer := strings.TrimSpace(string(typed))
			if answer == "" {
				answer = defaultValue
			}
			p.finish(r, restore, label, answer)
			return answer
		case k == keyBackspace && len(typed) > 0:
			typed = typed[:len(typed)-1]
		case c != 0:
			typed = append(typed, c)
		}
	}
}

// Confirm asks a yes or no question. --yes answers yes.
func (p *Printer) Confirm(label string, defaultValue bool) bool {
	if p.Yes {
		return true
	}
	if !p.Interactive {
		return defaultValue
	}
	restore, ok := p.enterRawMode()
	if !ok {
		return defaultValue
	}
	yes := defaultValue
	r := &region{out: p.Out}
	for {
		r.draw(p.box(label, []string{p.radio(yes, "Yes") + "   " + p.radio(!yes, "No")}, true))
		k, c, err := p.readKey()
		switch {
		case err != nil:
			restore()
			return defaultValue
		case k == keyCancel:
			p.cancelAndExit(restore)
		case k == keyEnter:
			answer := "No"
			if yes {
				answer = "Yes"
			}
			p.finish(r, restore, label, answer)
			return yes
		case k == keyLeft || k == keyRight || k == keyUp || k == keyDown || k == keySpace:
			yes = !yes
		case c == 'y' || c == 'Y':
			yes = true
		case c == 'n' || c == 'N':
			yes = false
		}
	}
}

func (p *Printer) radio(on bool, label string) string {
	if on {
		return p.Paint(Accent, "●") + " " + label
	}
	return p.Paint(Dim, "○ "+label)
}

// maxRows is how many options show at once. A longer list scrolls.
const maxRows = 10

// pickFromList shows a list. With multi it lets the user tick several items.
func (p *Printer) pickFromList(label string, options []string, selected int, multi bool) ([]int, bool) {
	if len(options) == 0 {
		return nil, true
	}
	restore, ok := p.enterRawMode()
	if !ok {
		return nil, false
	}
	checked := make([]bool, len(options))
	start := 0
	r := &region{out: p.Out}
	for {
		start = min(start, selected)
		start = max(start, selected-maxRows+1)
		var body []string
		for i := start; i < min(len(options), start+maxRows); i++ {
			body = append(body, p.optionLine(i == selected, options[i], multi, checked[i]))
		}
		r.draw(p.box(label, body, true))
		k, _, err := p.readKey()
		switch {
		case err != nil:
			restore()
			return nil, false
		case k == keyCancel:
			p.cancelAndExit(restore)
		case k == keyUp:
			selected = (selected - 1 + len(options)) % len(options)
		case k == keyDown:
			selected = (selected + 1) % len(options)
		case k == keySpace && multi:
			checked[selected] = !checked[selected]
		case k == keyEnter:
			var picked []int
			var names []string
			if multi {
				for i, on := range checked {
					if on {
						picked = append(picked, i)
						names = append(names, options[i])
					}
				}
			} else {
				picked = []int{selected}
				names = []string{options[selected]}
			}
			answer := strings.Join(names, ", ")
			if answer == "" {
				answer = "none"
			}
			p.finish(r, restore, label, answer)
			return picked, true
		}
	}
}

// optionLine draws one option. The arrow marks the line the cursor is on.
func (p *Printer) optionLine(current bool, label string, multi, checked bool) string {
	mark := "○"
	if multi {
		mark = "◻"
		if checked {
			mark = "◼"
		}
	} else if current {
		mark = "●"
	}
	if !current {
		return "  " + p.Paint(Dim, mark+" "+label)
	}
	return p.Paint(Accent, "›") + " " + p.Paint(Accent, mark) + " " + label
}

func (p *Printer) Choose(label string, options []string, defaultValue int) int {
	if !p.Interactive {
		return defaultValue
	}
	picked, ok := p.pickFromList(label, options, defaultValue, false)
	if !ok || len(picked) == 0 {
		return defaultValue
	}
	return picked[0]
}

func (p *Printer) Multi(label string, options []string) []int {
	if !p.Interactive {
		return nil
	}
	picked, _ := p.pickFromList(label, options, 0, true)
	return picked
}
