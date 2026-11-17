## 1. Check
- [x] 1.1 Add `spec_test.go` with `checkSpec` and a test that runs it on every spec. Check: `go test . -run Spec`
- [x] 1.2 Add a table test that proves `checkSpec` reports a missing `SHALL`, a missing scenario, a missing `Tests:` line and an unknown test name. Check: `go test . -run Spec`
- [x] 1.3 Run the full check. Check: `go vet ./... && go test ./...` and `gofmt -l .`

## Log
- 1.1: `spec_test.go` passes on the 5 specs. 1 test run, 0 retries.
- 1.2: table test covers 4 failure kinds and 2 passing cases. 1 test run, 0 retries.
- 1.3: vet, tests, gofmt and the empty-HOME run are green. Stop: no unchecked task.
