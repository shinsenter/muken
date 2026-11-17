// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	muken "github.com/shinsenter/muken"
	"github.com/shinsenter/muken/internal/ui"
)

func printLicense(p *ui.Printer, thirdParty bool) {
	if thirdParty {
		fmt.Fprint(p.Out, muken.ThirdParty)
		return
	}
	fmt.Fprintln(p.Out, muken.Notice)
	fmt.Fprintln(p.Out, muken.Trademark)
	fmt.Fprintln(p.Out, "The license text follows. Use `muken version --third-party` for the software inside muken.")
	fmt.Fprintln(p.Out)
	fmt.Fprint(p.Out, muken.License)
}
