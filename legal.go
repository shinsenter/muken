// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package muken holds the legal texts that are compiled into the binary,
// so `muken version --license` works even when the binary is the only file you have.
package muken

import _ "embed"

// License is the Apache License 2.0.
//
//go:embed LICENSE
var License string

// Notice is the copyright notice that the license asks you to keep.
//
//go:embed NOTICE
var Notice string

// Trademark says what you can do with the name.
//
//go:embed TRADEMARK.md
var Trademark string

// ThirdParty has the licenses of the software inside the binary.
// scripts/third-party-notices.sh writes the file.
//
//go:embed THIRD_PARTY_NOTICES.txt
var ThirdParty string
