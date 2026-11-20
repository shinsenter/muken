#!/bin/sh
# Run by the SessionStart hook. Updates the shinsenter/php submodule to the newest main,
# then compares it with the commit that the muken specs were last checked against
# (openspec/upstream.md). When they differ, it prints what changed. The text goes into the
# session as context, so the agent knows it must update the muken specs.
set -u

cd "$(git rev-parse --show-toplevel)"
sub=third_party/shinsenter-php

git submodule update --init --remote --depth 1 "$sub" >/dev/null 2>&1 ||
    echo "$sub: update failed, using the checked-out commit"

now="$(git -C "$sub" rev-parse HEAD 2>/dev/null)" || exit 0
seen="$(sed -n 's/^reviewed: *//p' openspec/upstream.md 2>/dev/null | head -1)"
[ "$now" = "$seen" ] && exit 0

echo "shinsenter/php changed since the muken specs were checked."
echo "  reviewed: ${seen:-none}"
echo "  now:      $now"
git -C "$sub" diff --stat "$seen" "$now" -- openspec src/webapps/laravel src/php/common docs 2>/dev/null | tail -15
echo "Task: read the changes in $sub/openspec and the Laravel and SSH docs, update the muken specs that depend on them"
echo "(image, api, web, history), then set 'reviewed:' in openspec/upstream.md to $now."
