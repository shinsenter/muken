#!/bin/sh
# Writes THIRD_PARTY_NOTICES.txt: the license text of the Go standard library and of every module
# that goes into the binary. The file is embedded, and `muken version --third-party` prints it.
# Run it again after you change src/go.mod, then commit both copies (the root file and src/legal/). A test fails when a module is missing.
set -eu

cd "$(git rev-parse --show-toplevel)"
out=THIRD_PARTY_NOTICES.txt
root="$PWD"
cd src

print_title() {
    echo
    echo "================================================================================"
    echo "$1"
    echo "================================================================================"
    echo
}

print_license() {
    found=0
    for name in LICENSE LICENSE.md LICENSE.txt COPYING NOTICE; do
        if [ -f "$1/$name" ]; then
            [ "$found" -eq 0 ] || echo
            cat "$1/$name"
            found=1
        fi
    done
    [ "$found" -eq 1 ] || echo "(no license file in this module, see its repository)"
}

# Go keeps LICENSE in GOROOT. Homebrew puts it one folder up.
go_license() {
    root="$(go env GOROOT)"
    for file in "$root/LICENSE" "$root/../LICENSE"; do
        if [ -f "$file" ]; then
            echo "$file"
            return
        fi
    done
    echo "cannot find the Go LICENSE file near $root" >&2
    exit 1
}

{
    echo "Third-party software in muken"
    echo
    echo "muken is built with the Go standard library and the modules below."
    echo "Each one is used under its own license, printed after its name."

    print_title "Go standard library $(go env GOVERSION)"
    cat "$(go_license)"

    # The modules differ a little per system (x/sys, for one), so list them for every release target.
    for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
        GOOS="${target%/*}" GOARCH="${target#*/}" go list -deps \
            -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' ./cmd/muken
    done |
        sort -u |
        while read -r path version dir; do
            print_title "$path $version"
            print_license "$dir"
        done
} >"$root/$out"
cp "$root/$out" "$root/src/legal/$out"

echo "wrote $out and src/legal/$out"
