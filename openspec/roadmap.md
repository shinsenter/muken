# Roadmap to 1.0

Target: 1.0 in early 2027. This is a proposal. Edit it.

## What 1.0 means

- The CLI is stable. Command names, flags and `muken.yaml` keys follow semver. A removal needs 2.0.
- Every feature in the guide has run on a real setup, or the guide says it is not supported.
- Release files and the image are signed and scanned, and `govulncheck` is clean at the tag.

## Exit criteria

Source: the section "What is tested and what is not" in `docs/guide.md`.

| # | Criterion | Now |
|---|---|---|
| 1 | Let's Encrypt and Cloudflare certificates work on a real domain | Not tested |
| 2 | Hetzner API: create and delete a server | Not tested |
| 3 | Cloudflare API: `dns sync`, `ssl_mode`, cache purge | Not tested |
| 4 | `agent install` and the public status page behind Traefik | Not tested |
| 5 | Several replicas, and a migration that the old version cannot use | Not tested |
| 6 | Workers and migrations, which replace the entrypoint with `sh -c` and run as `www-data` | Not tested |
| 7 | The app returns HTTP 500 until migrations run, while muken says "live" | Known gap. Fix it or document it |
| 8 | About 2 seconds of HTTP 502 after `maintenance off` | Known gap. Fix it or document it |
| 9 | Second `up`, spinner and summary output, `exec` without a terminal, on a real server | Unit tests only |
| 10 | CLI freeze: no more renames | Open |
| 11 | A user who is not the author reads the docs and finishes a first deploy | Open |

## Milestones

The order matters. The windows are suggestions.

| Milestone | Window | Content |
|---|---|---|
| 0.2 | Nov to Dec 2026 | Real-server runs for 1, 5, 6 and 9. Fix what breaks |
| 0.3 | Dec 2026 | Hetzner and Cloudflare real runs (2, 3). Known gaps 7 and 8 |
| 0.4 | Jan 2027 | Agent real run (4). Docs read by someone else (11) |
| 1.0-rc1 | Jan to Feb 2027 | CLI freeze (10). `govulncheck` clean. Only docs fixes left |
| 1.0 | Feb to Mar 2027 | Tag on `main`. Release |

## Until 1.0

- Work on the `0.x` branch. A breaking change uses `!` in the commit type and a line in `CHANGELOG.md`.
- Add no new command. A new feature needs a change in `openspec/changes/` that names the exit criterion it affects.
- A criterion is done when a run on a real setup is written down in the guide, with the date.

## Not in 1.0

Cloud Run. Several servers per app. A load balancer. Database backups. Migration rollback.

## Open questions

- Which accounts and domain to use for the real runs, and who pays for the servers?
- Who reads the docs for criterion 11?
- Which branch carries 1.x after the tag?
