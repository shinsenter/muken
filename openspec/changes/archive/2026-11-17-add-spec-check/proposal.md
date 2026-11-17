# add-spec-check

## Why
Specs name tests in their `Tests:` lines. An agent can invent a name or drop a `SHALL`, and nobody notices until a person reads the spec.

## What changes
- A test reads every `openspec/specs/*/spec.md`. It fails when a requirement has no `SHALL`, no scenario or no `Tests:` line, or when a `Tests:` line names a test that does not exist.

## Impact
- Specs: none
- Code: `spec_test.go`
- Roadmap: none
