// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/shinsenter/muken/internal/ui"
	"github.com/shinsenter/muken/legal"
)

func printLicense(p *ui.Printer, thirdParty bool) {
	if thirdParty {
		fmt.Fprint(p.Out, legal.ThirdParty)
		return
	}
	fmt.Fprintln(p.Out, legal.Notice)
	fmt.Fprintln(p.Out, legal.Trademark)
	fmt.Fprintln(p.Out, "The license text follows. Use `muken version --third-party` for the software inside muken.")
	fmt.Fprintln(p.Out)
	fmt.Fprint(p.Out, legal.License)
}
