# Loop

How to do a change with many tasks at a low token cost. The state lives in files, not in the chat.

## The loop

Run it inside one session. Do not use `/loop` for it. `/loop` is for waiting (see the end).

Repeat:

1. **Pick.** Take the first unchecked task in `openspec/changes/<id>/tasks.md`. None left: go to Stop.
2. **Read little.** Read the task, the delta spec and the files the task names. Use `grep -n` and line ranges. Do not read a file twice.
3. **Change.** Write the smallest change that satisfies the scenario.
4. **Check narrow.** Run the test of the package, with `-run`: `go test ./internal/deploy -run Maintenance 2>&1 | tail -20`.
5. **Pass.** Tick the task. Add one line to the log at the bottom of `tasks.md`. Go to 1.
6. **Fail.** Find the cause. Fix it. Go to 4. Count the attempt.

At the end of the change, run the full check once: `go vet ./... && go test ./... 2>&1 | grep -v '^ok'; gofmt -l .`

## Stop

Stop at the first of these. Report in 10 lines or fewer, result first.

| Condition | What to do |
|---|---|
| No unchecked task and the full check is green | Stop. Report. Suggest the commit groups |
| The same check fails 3 times with the same error | Stop. Give the error and two hypotheses. Do not try a fourth time |
| Two iterations in a row produce no diff | Stop. Say what blocks you |
| The task needs a decision that no spec makes: a new command, a flag, a dependency, a behavior change | Stop. Ask |
| The task needs a real server, a cloud API, Docker, the network or a secret | Stop. Ask with the reason and the command |
| A test would have to change to pass | Stop. Ask. Never edit a test to make it pass |
| The diff touches files outside the change | Stop. Report the files |
| The run did 8 tasks, or one task took more than 15 tool calls | Stop. Report progress and the next task |

## Keep the cost low

- Read the spec to learn behavior. Read code only for the lines you change.
- Cap output. End commands with `| tail -20` or `| grep -v '^ok'`.
- Run the narrowest test first. Run the full suite once per change, not per task.
- Do not repeat a plan or restate a result. One line per iteration.
- Send independent tool calls in one turn.
- Use `grep` and `ls` before you start a subagent. Use a subagent only for a large, independent read or review.
- Use a cheaper model for mechanical tasks (renames, docs, test scaffolding) and a stronger one for design and debugging. Switch with `/model`.
- Start one session per change. Run `/clear` between changes. The files carry the state.
- Commit by topic when the change is done, not per iteration.

## Resume

A new session resumes from the files. Read `tasks.md`: the first unchecked task is the next one, and the log says what happened.

## Use `/loop` for waiting

`/loop` re-enters the chat on a timer. It is for state that changes outside the session, such as CI. Put a stop condition and a cap in the prompt.

```text
/loop 10m Run `gh run list --branch 0.1 --limit 1`. Stop when the run is green, or when it fails. On failure, report the failing step in 5 lines. Stop after 12 checks.
```

Without an interval (`/loop <prompt>`), Claude picks the delay itself and ends the loop when the stop condition holds. Long delays (20 minutes or more) cost less than short ones. To end a loop early, tell Claude to stop it.
