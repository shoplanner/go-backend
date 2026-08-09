# syntax=docker/dockerfile:1

# Builds the backend against a Debian release and wraps it into a .deb.
#
# The build has to happen on the target release itself: the SQLite driver is cgo, so the binary
# ends up dynamically linked against the glibc of whatever built it. Doing it in the container
# means the host needs nothing but docker — no dpkg-deb, no Go, no matching libc.
#
# Everything under docs/ and every *.gen.go / sqlgen/ file is committed, so this deliberately
# runs a plain `go build` instead of `task build`: no swag, no sqlc, no python.
#
# The `deb` stage is scratch on purpose: `packaging/deb/build.sh` exports it with
# `--output type=local`, so the whole stage *is* the artifact directory.

# Debian release to build for: bookworm is 12, trixie is 13. Declared before the first FROM so
# it can select the base image; it has to be repeated inside the stage that uses it.
ARG DEBIAN_SUITE=bookworm

FROM golang:1.25.5-${DEBIAN_SUITE} AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# The package, not cmd/backend/main.go: newLogWriter lives in the build-tagged logger_*.go
# siblings, which a single-file build would leave out.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 go build -ldflags="-w -s" -o bin/backend ./cmd/backend

FROM build AS package

# Debian version of the package: @VERSION@-@REVISION@~@SUITE@. Set by packaging/deb/build.sh.
ARG VERSION=0.0.0
ARG REVISION=1
ARG DEBIAN_SUITE

COPY packaging/deb/ /packaging/

# Two values are read out of the build rather than passed in, so that they always describe what
# was actually produced:
#   - the architecture, which follows --platform;
#   - the libc6 floor, taken from the highest GLIBC_* symbol version the binary imports. Cgo
#     makes that depend on the base image, and a hand-written constant would silently rot the
#     moment DEBIAN_SUITE changes.
RUN set -eu; \
    arch="$(dpkg --print-architecture)"; \
    libc="$(objdump -T bin/backend | sed -n 's/.*GLIBC_\([0-9.]*\).*/\1/p' | sort -V | tail -1)"; \
    : "${libc:?could not determine the glibc version required by bin/backend}"; \
    version="${VERSION}-${REVISION}~${DEBIAN_SUITE}"; \
    root="/build/shoplanner-backend"; \
    mkdir -p \
        "$root/DEBIAN" \
        "$root/usr/bin" \
        "$root/usr/libexec/shoplanner" \
        "$root/lib/systemd/system" \
        "$root/etc/shoplanner" \
        /out; \
    install -m 0755 bin/backend "$root/usr/bin/shoplanner"; \
    install -m 0755 /packaging/generate-key.sh "$root/usr/libexec/shoplanner/generate-key.sh"; \
    install -m 0644 config/systemd/shoplanner.service "$root/lib/systemd/system/shoplanner.service"; \
    install -m 0644 config/backend.yml "$root/etc/shoplanner/backend.yml"; \
    sed -e "s/@VERSION@/${version}/" -e "s/@ARCH@/${arch}/" -e "s/@LIBC@/${libc}/" \
        /packaging/control.in > "$root/DEBIAN/control"; \
    install -m 0644 /packaging/conffiles "$root/DEBIAN/conffiles"; \
    install -m 0755 /packaging/postinst "$root/DEBIAN/postinst"; \
    install -m 0755 /packaging/prerm "$root/DEBIAN/prerm"; \
    install -m 0755 /packaging/postrm "$root/DEBIAN/postrm"; \
    dpkg-deb --build --root-owner-group "$root" \
        "/out/shoplanner-backend_${version}_${arch}.deb"; \
    dpkg-deb --info /out/*.deb; \
    dpkg-deb --contents /out/*.deb

FROM scratch AS deb
COPY --from=package /out/ /
