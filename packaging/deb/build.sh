#!/usr/bin/env bash
# Builds the Debian package into dist/.
#
# The compile runs inside a container of the target release: the binary is cgo-linked against
# glibc, so it has to be built on the distribution it targets. That means the host needs nothing
# but docker with buildx — no dpkg-deb, no Go toolchain.
#
# Environment:
#   DEBIAN_SUITE  release to build for; bookworm (12, default) or trixie (13)
#   VERSION       upstream version; defaults to the git commit, see git_version below
#   REVISION      Debian revision, defaults to 1
#   OUT_DIR       where the .deb lands, defaults to dist
#   PLATFORM      target platform, e.g. linux/arm64; defaults to the host (foreign ones need qemu)
set -euo pipefail

cd "$(dirname "$0")/../.."

# The package version is derived from the commit: <commits on HEAD>.g<short sha>, e.g.
# 49.gb344fdc, plus a .dirty suffix when the tree has uncommitted changes.
#
# The commit count leads because a Debian version has to start with a digit and, more to the
# point, has to sort: dpkg compares it numerically, so every later commit produces a version apt
# sees as an upgrade. The sha alone would not order at all. Tags are deliberately not consulted,
# so a build never depends on whether tags were fetched.
git_version() {
    local count sha dirty=""

    count="$(git rev-list --count HEAD)"
    sha="$(git rev-parse --short=7 HEAD)"

    if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
        dirty=".dirty"
    fi

    printf '%s.g%s%s\n' "$count" "$sha" "$dirty"
}

DEBIAN_SUITE="${DEBIAN_SUITE:-bookworm}"
VERSION="${VERSION:-$(git_version)}"
REVISION="${REVISION:-1}"
OUT_DIR="${OUT_DIR:-dist}"

platform_arg=()
if [[ -n "${PLATFORM:-}" ]]; then
    platform_arg=(--platform "$PLATFORM")
fi

# The suite goes into the Debian revision as ~bookworm / ~trixie, the usual way of saying "same
# source, built for that release": it keeps two builds of one commit from overwriting each other
# in $OUT_DIR, and '~' sorts below a plain revision so neither shadows a future real release.
echo ">> building shoplanner-backend ${VERSION}-${REVISION}~${DEBIAN_SUITE}"

# The `deb` stage is scratch and holds nothing but the package, so exporting it as a local
# directory drops the .deb straight into $OUT_DIR.
docker buildx build \
    --file packaging/deb/Dockerfile \
    --target deb \
    --build-arg "DEBIAN_SUITE=$DEBIAN_SUITE" \
    --build-arg "VERSION=$VERSION" \
    --build-arg "REVISION=$REVISION" \
    "${platform_arg[@]}" \
    --output "type=local,dest=$OUT_DIR" \
    .

ls -l "$OUT_DIR"/shoplanner-backend_"${VERSION}-${REVISION}~${DEBIAN_SUITE}"_*.deb
