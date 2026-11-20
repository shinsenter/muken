# Upstream

A digest of [shinsenter/php](https://github.com/shinsenter/php), so a session reads this file and not the submodule. The submodule is read-only, in `third_party/shinsenter-php`. Open it only for a detail that is missing here.

## Keep it current

The `SessionStart` hook (`.claude/settings.json`) runs `src/scripts/sync-upstream.sh`. It updates the submodule and prints a task when the commit differs from `reviewed:` below. When you get that task:

1. Read what changed in `third_party/shinsenter-php/openspec/specs`, `src/webapps/laravel` and `src/php/common`.
2. Update the digest below.
3. Update the muken specs that depend on it: `specs/image`, and `changes/add-web-ui` until it is archived (then `specs/api`, `specs/web`, `specs/history`).
4. Set `reviewed:` to the new commit.

reviewed: 49c8243607be0330ea9e6140fb93fecb5d42c4ef

## Digest

Checked in the submodule at the commit above. "Unverified" means the text does not say it.

| Topic | Fact |
|---|---|
| Image | `shinsenter/laravel`, built on `shinsenter/phpfpm-nginx` (Nginx, PHP-FPM, Composer). No database inside |
| Tags | `latest`. The README says to add `-alpine` for the small variant. `latest-alpine` exists (pulled 2026-11-20, PHP 8.5.12 then) |
| Paths | App in `/var/www/html` (`APP_PATH`), served from `public` (`DOCUMENT_ROOT`). Runs as `www-data` (`APP_USER`) |
| Ports | 80, 443 and 443/udp. Self-signed certificate in `/etc/ssl/site/server.crt` and `server.key` |
| SSH | `ENABLE_SSHD=1` turns it on. `SSHD_AUTHORIZED_KEYS` (one key per line) replaces `~/.ssh/authorized_keys` at every start. `SSHD_USER` defaults to `$APP_USER`, `SSHD_PORT` to 22. Password login is off. If `sshd` is missing, the hook `/etc/hooks/post-boot/00-ssh-server` runs `pkg-add openssh-server` at boot, which needs the network |
| New project | An empty `/var/www/html` makes the first start run `composer create-project $INITIAL_PROJECT` (default `laravel/laravel`) |
| Migration | `LARAVEL_AUTO_MIGRATION` defaults to `1`: `artisan migrate` at boot. `LARAVEL_AUTO_MIGRATION_OPTIONS` adds flags. Whether it creates a missing SQLite file is Unverified |
| Switches | `LARAVEL_ENABLE_SCHEDULER` default `1`. `LARAVEL_ENABLE_QUEUE_WORKER`, `_HORIZON`, `_PULSE`, `_REVERB` default `0`. Each has `..._OPTIONS` |
| Boot | Scripts in `/etc/hooks/post-boot/`. `DISABLE_AUTORUN_SCRIPTS=1` turns them all off. `DEBUG=1` prints which hooks run. Services run under s6 (`s6-service <name> longrun`) |
| Hooks | Folders under `/etc/hooks/`: `post-boot`, `application`, `onready`, `onlive`, `cleanup`. The Laravel image has `application/init-laravel`: when `.env` has an empty `APP_KEY=` it runs `artisan key:generate`, when it has `DB_CONNECTION=sqlite` it touches `database/database.sqlite`, then `artisan migrate --force` if `LARAVEL_AUTO_MIGRATION` is on, then `storage:link`. Helpers: `artisan`, `web-do` (run as the app user), `s6-service <name> longrun\|enable\|disable`, `is-true`, `has-cmd`, `has-s6`, `debug-echo` |
| Tag name | The upstream Dockerfile builds `${BUILD_FROM_IMAGE}:${BUILD_TAG_PREFIX}php${PHP_VERSION}${PHP_VARIANT}`. The README shows `shinsenter/laravel:latest` |
| Aliases | `php`, `composer`, `artisan` are shell aliases in `/usr/local/aliases` that run `in-app <real binary>`, which works in `APP_PATH` (`/var/www/html`) and ignores the current folder and `WORKDIR`. To work in another folder, set `APP_PATH` to it (the muken build stages use `APP_PATH=/work`). Node and npm are not in the image |
| Other | `TZ`, `PHP_*` (php.ini, list with `php-envvars`), `ENABLE_CRONTAB` |
| Rules for edits there | No `docker build` without asking. Specs say "Unverified" when untested |
