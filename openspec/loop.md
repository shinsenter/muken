# Loop

How to do a change with many tasks at a low token cost. The state lives in files, not in the chat.

## The loop

Run it inside one session. Do not use `/loop` for it. `/loop` is for waiting (see the end).

Repeat:

1. **Pick.** Take the first unchecked task in `openspec/changes/<id>/tasks.md`. None left: go to Stop.
2. **Read little.** Read the `Context` block of `tasks.md`, then the files the task names. Use `grep -n` and line ranges. Do not read a file twice.
3. **Change.** Write the smallest change that satisfies the scenario.
4. **Check narrow.** `make t PKG=./internal/deploy RUN=Maintenance`.
5. **Pass.** Tick the task. Add one line to the log. Go to 1.
6. **Fail.** Find the cause. Fix it. Go to 4. Count the attempt.

At the end of the change run `make check` once. Run `make check-ci` before you hand over.

## Stop

Stop at the first of these. Report in 10 lines or fewer, result first.

| Condition | What to do |
|---|---|
| No unchecked task and `make check` is green | Stop. Report. Suggest the commit groups |
| The same check fails 3 times with the same error | Stop. Give the error and two hypotheses. Do not try a fourth time |
| Two iterations in a row produce no diff | Stop. Say what blocks you |
| The task needs a decision that no spec makes: a new command, a flag, a dependency, a behavior change | Stop. Ask |
| The task needs a real server, a cloud API, Docker, the network or a secret | Stop. Ask with the reason and the command |
| A test would have to change to pass | Stop. Ask. Never edit a test to make it pass |
| The diff touches files outside the change | Stop. Report the files |
| The run did 8 tasks, or one task took more than 15 tool calls | Stop. Report progress and the next task |

## Keep the context small

- Load in this order and stop: `AGENTS.md` (automatic), `openspec/index.md`, one spec, the `Context` block. Nothing else.
- Never read `dist/`, `THIRD_PARTY_NOTICES.txt`, `LICENSE` or `NOTICE`. Never read all of `docs/guide.md`.
- Do not edit `AGENTS.md`, `CLAUDE.md` or another file that loads at the start of a session while a run is going. An edit changes the start of the prompt, so the cache misses and every later turn costs full price.
- Run checks with `make check` and `make t`. They print only problems.
- Send independent tool calls in one turn.
- A search that needs more than 3 files goes to a subagent. It returns a summary, and the raw reads stay with it.
- One session per change. Run `/clear` between changes. To go on in a long session, run `/compact keep the task list and the failing check`.
- End every iteration with the file state: tick, log line. The last log line starts with `Next:` and names the next task or `none`. A new session reads `tasks.md` and nothing else.
- Pick the model by task with `/model`: a cheap one for renames, docs and test scaffolding, a strong one for design and debugging.
- Commit by topic when the change is done, not per iteration.

## Use `/loop` for waiting

`/loop` re-enters the chat on a timer. It is for state that changes outside the session, such as CI. Put a stop condition and a cap in the prompt.

```text
/loop 10m Run `gh run list --branch 0.1 --limit 1`. Stop when the run is green, or when it fails. On failure, report the failing step in 5 lines. Stop after 12 checks.
```

Without an interval (`/loop <prompt>`), Claude picks the delay itself and ends the loop when the stop condition holds. Long delays (20 minutes or more) cost less than short ones. To end a loop early, tell Claude to stop it.
