# OpenSpec workflow

Specs describe what muken does now. Changes describe what it will do next. Code follows the specs.

## Layout

```
openspec/
  project.md                 context, conventions, decisions
  roadmap.md                 plan to 1.0
  loop.md                    how to run many tasks with a low token cost
  specs/<capability>/spec.md what the system does now
  changes/<change-id>/       work in progress
    proposal.md              why and what
    tasks.md                 checklist
    design.md                only for a hard choice
    specs/<capability>/spec.md   the delta: ADDED, MODIFIED, REMOVED
  changes/archive/           finished changes, named YYYY-MM-DD-<change-id>
```

## When to write a change

| Write a change for | Skip it for |
|---|---|
| A new command, flag or `muken.yaml` key | A bug fix that restores what a spec already says |
| A behavior change or a removal | A typo, a comment, a rename with no behavior change |
| A new dependency, a new server-side effect | Docs only, tests for existing behavior |
| A change that can interrupt a running app | A dependency bump with no behavior change |

Unsure? Write the change. It costs one small file.

## Steps

1. Read `project.md`, the spec of the capability, and `ls openspec/changes` for work in progress.
2. Create `changes/<change-id>/`. The id is kebab-case and starts with a verb: `add-`, `update-`, `remove-`, `refactor-`.
3. Write `proposal.md`, `tasks.md` and the delta specs. Keep each short.
4. Stop. Show the user the proposal and wait for a clear GO.
5. Do the tasks in order. Follow `loop.md`. Tick each task when its check passes.
6. Update the `Tests:` line of every requirement you touched.
7. Archive: move the folder to `changes/archive/YYYY-MM-DD-<change-id>/` and merge the deltas into `specs/`.

## Spec format

```markdown
# <Capability> Specification

## Purpose
One sentence.

## Requirements

### Requirement: <name>
The system SHALL <behavior>.

#### Scenario: <name>
- **WHEN** <condition>
- **THEN** <result>
- **AND** <result>

Tests: `TestName`, `TestOther`
```

- Every requirement has a `SHALL` and at least one scenario.
- Write what a user or a test can observe. Do not write how the code does it.
- `Tests:` lists real test names. Write `Tests: none yet` when there is none. Never invent a name.

## Delta format

A delta spec under `changes/<id>/specs/<capability>/spec.md` has one or more of these headings:

- `## ADDED Requirements`
- `## MODIFIED Requirements`: repeat the whole requirement with the new text.
- `## REMOVED Requirements`: name the requirement and say why.

## proposal.md

```markdown
# <change-id>

## Why
One or two sentences.

## What changes
- Bullet per visible change. Mark breaking changes with **BREAKING**.

## Impact
- Specs: <capabilities>
- Code: <paths>
- Roadmap: <exit criteria it touches, or none>
```

## tasks.md

```markdown
## 1. <group>
- [ ] 1.1 <one small task, with the check that proves it>
```

A task fits in one iteration. If it does not, split it.

## Tools

If the `openspec` CLI is installed, `openspec validate <change-id> --strict` checks the format. It is optional. The checks that decide are the tests in `AGENTS.md`.
