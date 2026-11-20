// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package ui

// The colors of the console output. They are ANSI codes for Printer.Paint.
// Change a color here and it changes everywhere.
const (
	// Accent is a light yellow green from the 256 color palette. It marks
	// success, the active prompt, command names and section titles.
	Accent = "38;5;154"
	// Heading is the section title: the accent color in bold.
	Heading = "1;" + Accent
	// Warning is yellow: a warning, a hint, a state that needs a look.
	Warning = "33"
	// Danger is red: an error or a service that is down.
	Danger = "31"
	// Dim is gray text: frames, dots, labels and answered prompts.
	Dim = "2"
)
