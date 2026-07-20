#!/usr/bin/env bash
set -euo pipefail

# Renders packaging/aur/PKGBUILD.template for a release.
#   gen-aur-pkgbuild.sh <tagver> <sha256>   e.g. 0.1.0-beta.1 <hash>
# pkgver must satisfy pacman's vercmp: 0.1.0beta1 < 0.1.0 (dots or hyphens
# in the prerelease suffix would sort NEWER than the final release).

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TAGVER="${1:?usage: gen-aur-pkgbuild.sh <tagver> <sha256>}"
SHA256="${2:?usage: gen-aur-pkgbuild.sh <tagver> <sha256>}"
PKGVER="$(echo "$TAGVER" | sed -E 's/-(alpha|beta|rc)\.?([0-9]+)/\1\2/')"

sed -e "s/@PKGVER@/$PKGVER/" \
    -e "s/@TAGVER@/$TAGVER/" \
    -e "s/@SHA256@/$SHA256/" \
    "$ROOT/packaging/aur/PKGBUILD.template"
